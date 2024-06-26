package dns

import (
	"net"
	"strings"
	"time"

	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/component/fakeip"
	"github.com/Dreamacro/clash/component/trie"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/context"
	"github.com/Dreamacro/clash/log"

	D "github.com/miekg/dns"
)

type (
	handler    func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error)
	middleware func(next handler) handler
)

func withHosts(hosts *trie.DomainTrie) middleware {
	return func(next handler) handler {
		return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
			q := r.Question[0]

			if !isIPorPTRRequest(q) {
				return next(ctx, r)
			}

			record := hosts.Search(strings.ToLower(strings.TrimRight(q.Name, ".")))
			if record == nil {
				return next(ctx, r)
			}

			msg := r.Copy()
			switch data := record.Data.(type) {
			case []net.IP:
				for _, ip := range data {
					switch q.Qtype {
					case D.TypeA:
						if v4 := ip.To4(); v4 != nil {
							rr := &D.A{}
							rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeA, Class: D.ClassINET, Ttl: 60}
							rr.A = v4

							msg.Answer = append(msg.Answer, rr)
						}
					case D.TypeAAAA:
						if v6 := ip.To16(); v6 != nil {
							rr := &D.AAAA{}
							rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeAAAA, Class: D.ClassINET, Ttl: 60}
							rr.AAAA = v6

							msg.Answer = append(msg.Answer, rr)
						}
					}
				}
			case []string:
				for _, ptr := range data {
					switch q.Qtype {
					case D.TypePTR:
						rr := &D.PTR{}
						rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypePTR, Class: D.ClassINET, Ttl: 60}
						rr.Ptr = ptr

						msg.Answer = append(msg.Answer, rr)
					default:
						return handleMsgWithEmptyAnswer(r), nil
					}
				}
			}

			if len(msg.Answer) > 0 {
				ctx.SetType(context.DNSTypeHost)
				msg.SetRcode(r, D.RcodeSuccess)
				msg.Authoritative = true
				msg.RecursionAvailable = true

				return msg, nil
			}

			return next(ctx, r)
		}
	}
}

func withMapping(mapping *cache.LruCache) middleware {
	return func(next handler) handler {
		return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
			q := r.Question[0]

			if !isIPRequest(q) {
				return next(ctx, r)
			}

			msg, err := next(ctx, r)
			if err != nil {
				return nil, err
			}

			host := strings.TrimRight(q.Name, ".")

			for _, ans := range msg.Answer {
				var ip net.IP
				var ttl uint32

				switch a := ans.(type) {
				case *D.A:
					ip = a.A
					ttl = a.Hdr.Ttl
				case *D.AAAA:
					ip = a.AAAA
					ttl = a.Hdr.Ttl
				default:
					continue
				}

				mapping.SetWithExpire(ip.String(), host, time.Now().Add(time.Second*time.Duration(ttl)))
			}

			return msg, nil
		}
	}
}

func withFakeIP(fakePool *fakeip.Pool) middleware {
	return func(next handler) handler {
		return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
			q := r.Question[0]

			host := strings.TrimRight(q.Name, ".")
			if fakePool.ShouldSkipped(host) {
				return next(ctx, r)
			}

			switch q.Qtype {
			case D.TypeAAAA, D.TypeSVCB, D.TypeHTTPS:
				return handleMsgWithEmptyAnswer(r), nil
			}

			if q.Qtype != D.TypeA {
				return next(ctx, r)
			}

			rr := &D.A{}
			rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
			ip := fakePool.Lookup(host)
			rr.A = ip
			msg := r.Copy()
			msg.Answer = []D.RR{rr}

			ctx.SetType(context.DNSTypeFakeIP)
			setMsgTTL(msg, 1)
			msg.SetRcode(r, D.RcodeSuccess)
			msg.Authoritative = true
			msg.RecursionAvailable = true

			return msg, nil
		}
	}
}

func withResolver(resolver *Resolver) handler {
	return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
		ctx.SetType(context.DNSTypeRaw)
		q := r.Question[0]

		// return a empty AAAA msg when ipv6 disabled
		if !resolver.ipv6 && q.Qtype == D.TypeAAAA {
			return handleMsgWithEmptyAnswer(r), nil
		}

		msg, err := resolver.Exchange(r)
		if err != nil {
			log.Debugln("[DNS Server] Exchange %s failed: %v", q.String(), err)
			return msg, err
		}
		msg.SetRcode(r, msg.Rcode)
		msg.Authoritative = true

		return msg, nil
	}
}

func compose(middlewares []middleware, endpoint handler) handler {
	length := len(middlewares)
	h := endpoint
	for i := length - 1; i >= 0; i-- {
		middleware := middlewares[i]
		h = middleware(h)
	}

	return h
}

func newHandler(resolver *Resolver, mapper *ResolverEnhancer) handler {
	middlewares := []middleware{}

	if resolver.localHosts != nil {
		middlewares = append(middlewares, withHosts(resolver.localHosts))
	}

	if resolver.hosts != nil {
		middlewares = append(middlewares, withHosts(resolver.hosts))
	}

	if mapper.mode == C.DNSFakeIP {
		middlewares = append(middlewares, withFakeIP(mapper.fakePool))
	}

	if mapper.mode != C.DNSNormal {
		middlewares = append(middlewares, withMapping(mapper.mapping))
	}

	return compose(middlewares, withResolver(resolver))
}
