package httpkit

import (
	"strconv"
	"strings"
)

// Status is what an HTTP status code means and what to do about it.
type Status struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
	Cause   string `json:"cause"` // the usual reason you see it
	Retry   string `json:"retry"` // yes, no, or when
}

// statuses are the codes in RFC 9110 and the others seen in practice
// (proxies, gateways, Kubernetes ingresses, cloud load balancers).
var statuses = []Status{
	{100, "Continue", "The server got the request headers; send the body.", "The client sent Expect: 100-continue before a large upload.", "Not an error."},
	{101, "Switching Protocols", "The server is changing protocol as asked.", "A WebSocket or HTTP/2 upgrade.", "Not an error."},
	{103, "Early Hints", "Headers to act on before the final response.", "The server lets the browser preload resources early.", "Not an error."},
	{200, "OK", "The request succeeded.", "", "Not an error."},
	{201, "Created", "The request succeeded and made a new resource.", "A POST or PUT that created something; Location often says where.", "Not an error."},
	{202, "Accepted", "Taken for processing, not finished.", "The work runs in the background; poll for the result.", "Not an error."},
	{203, "Non-Authoritative Information", "Success, but a proxy changed the content.", "A transforming proxy in between.", "Not an error."},
	{204, "No Content", "Success, with no body.", "A DELETE or an update that returns nothing.", "Not an error."},
	{205, "Reset Content", "Success; clear the form or view.", "", "Not an error."},
	{206, "Partial Content", "Part of the resource, as asked with Range.", "A resumed or ranged download.", "Not an error."},
	{207, "Multi-Status", "Several results for several resources (WebDAV).", "", "Not an error."},
	{300, "Multiple Choices", "Several representations; pick one.", "", "Follow one of the links."},
	{301, "Moved Permanently", "The resource has a new URL for good.", "A renamed path, or http:// sent to https://. Update the URL.", "Follow Location (curl -L)."},
	{302, "Found", "The resource is at another URL for now.", "A login redirect, or a temporary move.", "Follow Location (curl -L)."},
	{303, "See Other", "Get the result from another URL with GET.", "After a POST, the server points at the result.", "Follow Location with GET."},
	{304, "Not Modified", "The cached copy is still good.", "A conditional request (If-None-Match, If-Modified-Since).", "Not an error; use the cache."},
	{307, "Temporary Redirect", "Repeat the same request at another URL for now.", "Like 302, but the method and body must stay the same.", "Follow Location (curl -L)."},
	{308, "Permanent Redirect", "Repeat the same request at another URL, for good.", "Like 301, but the method and body must stay the same.", "Follow Location (curl -L)."},
	{400, "Bad Request", "The server can't read the request.", "Malformed JSON, a missing or wrong field, a bad query string, or a header too large.", "No: fix the request."},
	{401, "Unauthorized", "Credentials are missing or not accepted.", "No Authorization header, an expired token, or a token for another audience. WWW-Authenticate says what is expected.", "No: get a fresh token, then retry."},
	{402, "Payment Required", "Reserved; some APIs use it for billing limits.", "A plan or quota that has run out.", "No."},
	{403, "Forbidden", "The server knows who you are, and says no.", "Missing permission or role, an IP allow list, a WAF rule, or a CSRF check.", "No: get the permission."},
	{404, "Not Found", "Nothing at this URL (or it is hidden from you).", "A wrong path or ID, a missing route in the ingress, or an API version that doesn't exist.", "No."},
	{405, "Method Not Allowed", "The URL exists but not with this method.", "POST to a GET-only route, or the other way round. Allow lists the methods.", "No: use an allowed method."},
	{406, "Not Acceptable", "Nothing matches the Accept header.", "Accept asks for a type the server doesn't make.", "No: change Accept."},
	{407, "Proxy Authentication Required", "The proxy wants credentials.", "A corporate proxy; set the proxy user.", "No: authenticate to the proxy."},
	{408, "Request Timeout", "The server gave up waiting for the request.", "A slow client or upload, or an idle keep-alive connection closed.", "Yes."},
	{409, "Conflict", "The request clashes with the resource's state.", "An edit conflict, a duplicate name, or an optimistic-lock version mismatch (Kubernetes: the object was modified).", "After fetching the latest state."},
	{410, "Gone", "The resource was here and is gone for good.", "Deleted, or an API version that was removed.", "No."},
	{411, "Length Required", "Content-Length is needed.", "A body sent without its length.", "No: send Content-Length."},
	{412, "Precondition Failed", "An If-Match or similar condition is false.", "The resource changed since you read it.", "After fetching the latest version."},
	{413, "Content Too Large", "The body is bigger than the server takes.", "An upload over the limit, e.g. nginx client_max_body_size (1 MB by default in ingress-nginx).", "No: send less, or raise the limit."},
	{414, "URI Too Long", "The URL is longer than the server takes.", "Too much in the query string; send it in a POST body.", "No."},
	{415, "Unsupported Media Type", "The server can't take this Content-Type.", "A JSON body without Content-Type: application/json, or the wrong encoding.", "No: fix Content-Type."},
	{416, "Range Not Satisfiable", "The Range asked for is outside the resource.", "Resuming a download that has changed or is shorter.", "No."},
	{417, "Expectation Failed", "The Expect header can't be met.", "", "No: drop Expect."},
	{418, "I'm a teapot", "An April Fools' joke (RFC 2324), used by some as a refusal.", "", "No."},
	{421, "Misdirected Request", "This server can't answer for this host.", "A reused HTTP/2 connection to the wrong host, or SNI and Host disagree.", "Yes, on a new connection."},
	{422, "Unprocessable Content", "The request is readable but its content is wrong.", "Validation failed: a field out of range, a missing required value.", "No: fix the data."},
	{423, "Locked", "The resource is locked (WebDAV).", "", "Later."},
	{424, "Failed Dependency", "Failed because an earlier request failed (WebDAV).", "", "No."},
	{425, "Too Early", "The server won't risk a replayed early request.", "TLS 1.3 early data.", "Yes, after the handshake."},
	{426, "Upgrade Required", "Switch to another protocol first.", "The server wants TLS or a newer HTTP.", "No: upgrade."},
	{428, "Precondition Required", "The request must be conditional.", "Send If-Match to avoid lost updates.", "No: add the condition."},
	{429, "Too Many Requests", "Rate limited.", "Too many requests in a time window, per key, user or IP.", "Yes, after Retry-After, with backoff."},
	{431, "Request Header Fields Too Large", "The headers are too big.", "Large cookies or tokens.", "No: send smaller headers."},
	{451, "Unavailable For Legal Reasons", "Blocked for legal reasons.", "", "No."},
	{499, "Client Closed Request", "nginx: the client hung up before the answer.", "A client timeout shorter than the server's work; seen in ingress-nginx logs.", "Yes, with a longer client timeout."},
	{500, "Internal Server Error", "The server failed on its side.", "An unhandled error or panic in the app; its logs say why.", "Maybe: if the request is idempotent."},
	{501, "Not Implemented", "The server doesn't support this method or feature.", "", "No."},
	{502, "Bad Gateway", "A proxy got a bad or no answer from the app behind it.", "The app crashed or restarted, closed the connection, or listens on another port; in Kubernetes, a pod not ready or a wrong targetPort.", "Yes, with backoff."},
	{503, "Service Unavailable", "The server can't take requests right now.", "Overloaded, in maintenance, or no healthy backends (no ready pods behind the Service).", "Yes, after Retry-After, with backoff."},
	{504, "Gateway Timeout", "A proxy waited too long for the app behind it.", "A slow query or call; the proxy's timeout (ingress proxy-read-timeout, load balancer idle timeout) is shorter than the work.", "Yes, if idempotent; look at the slow call."},
	{505, "HTTP Version Not Supported", "The server doesn't speak this HTTP version.", "", "No."},
	{507, "Insufficient Storage", "The server is out of space (WebDAV).", "", "Later."},
	{508, "Loop Detected", "The server found an infinite loop (WebDAV).", "", "No."},
	{511, "Network Authentication Required", "Sign in to the network first.", "A captive portal (hotel or airport Wi-Fi).", "After signing in."},
	{520, "Web Server Returned an Unknown Error", "Cloudflare: the origin sent something unexpected.", "An empty or invalid response from the origin.", "Yes."},
	{521, "Web Server Is Down", "Cloudflare: the origin refused the connection.", "The origin server is down or blocks Cloudflare.", "Yes, later."},
	{522, "Connection Timed Out", "Cloudflare: no TCP connection to the origin.", "The origin is unreachable or overloaded.", "Yes, later."},
	{524, "A Timeout Occurred", "Cloudflare: the origin took over 100 seconds to answer.", "A long-running request; make it asynchronous.", "Maybe."},
}

// Statuses finds codes by number ("404", "5" for every 5xx, "50" for
// 500–509) or by words in the name, meaning or cause. An empty query is
// every code.
func Statuses(q string) []Status {
	q = strings.ToLower(strings.TrimSpace(q))
	out := []Status{}
	if q == "" {
		return append(out, statuses...)
	}
	if _, err := strconv.Atoi(q); err == nil && len(q) <= 3 {
		for _, s := range statuses {
			if strings.HasPrefix(strconv.Itoa(s.Code), q) {
				out = append(out, s)
			}
		}
		return out
	}
	words := strings.Fields(q)
	for _, s := range statuses {
		text := strings.ToLower(s.Name + " " + s.Meaning + " " + s.Cause)
		all := true
		for _, w := range words {
			if !strings.Contains(text, w) {
				all = false
				break
			}
		}
		if all {
			out = append(out, s)
		}
	}
	return out
}
