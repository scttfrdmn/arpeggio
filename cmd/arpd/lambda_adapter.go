package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// newLambdaAdapter bridges API Gateway HTTP API (payload format v2) to
// net/http, so the same handler serves both Lambda and local development.
//
// The library ships no net/http bridge for APIGWv2 — lambdaurl.Wrap targets
// Lambda Function URLs in RESPONSE_STREAM mode, a different event shape — and
// pulling in a proxy library would add a dependency for a dozen lines. So this
// adapter is by hand: build an *http.Request from the event, record the
// handler's response, and translate it back.
func newLambdaAdapter(h http.Handler) func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		body := []byte(req.Body)
		if req.IsBase64Encoded {
			decoded, err := base64.StdEncoding.DecodeString(req.Body)
			if err != nil {
				return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusBadRequest}, nil
			}
			body = decoded
		}

		target := req.RawPath
		if req.RawQueryString != "" {
			target += "?" + req.RawQueryString
		}

		r := httptest.NewRequestWithContext(ctx, req.RequestContext.HTTP.Method, target, bytes.NewReader(body))
		for k, v := range req.Headers {
			r.Header.Set(k, v)
		}
		// APIGWv2 delivers cookies in a dedicated array, not the Headers map.
		// Rejoin them so handlers reading r.Cookie(...) behave as they do
		// against the local server.
		if len(req.Cookies) > 0 {
			r.Header.Set("Cookie", strings.Join(req.Cookies, "; "))
		}

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)

		resp := events.APIGatewayV2HTTPResponse{
			StatusCode: rec.Code,
			Headers:    make(map[string]string, len(rec.Header())),
		}
		for k, vs := range rec.Header() {
			// Set-Cookie must ride in the Cookies field; API Gateway emits one
			// Set-Cookie header per entry. Folding it into Headers would drop
			// all but one and corrupt multi-cookie responses.
			if http.CanonicalHeaderKey(k) == "Set-Cookie" {
				resp.Cookies = append(resp.Cookies, vs...)
				continue
			}
			resp.Headers[k] = strings.Join(vs, ", ")
		}

		if b := rec.Body.Bytes(); len(b) > 0 {
			resp.Body = string(b)
		}
		return resp, nil
	}
}
