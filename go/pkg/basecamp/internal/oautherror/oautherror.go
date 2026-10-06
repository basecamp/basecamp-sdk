// Package oautherror classifies a token endpoint's non-200 response — the one
// rule shared by the oauth package's Exchanger and the basecamp package's
// AuthManager refresh, which cannot import each other.
package oautherror

import "encoding/json"

// authRequiredCodes are the OAuth error codes a caller resolves by signing in
// again: the grant is invalid, expired or revoked (invalid_grant), the client
// failed to authenticate (invalid_client) or may not use this grant
// (unauthorized_client), or the resource owner refused (access_denied).
// Classified by code, not status: RFC 6749 §5.2 answers invalid_client with a
// 400 unless the client authenticated through the Authorization header, so a
// 401 cannot be the only signal. Matches every other SDK's token endpoint
// (conformance/oauth-token).
var authRequiredCodes = map[string]bool{
	"invalid_grant":       true,
	"invalid_client":      true,
	"unauthorized_client": true,
	"access_denied":       true,
}

// Parse reads the RFC 6749 error and error_description members of a token
// endpoint response body. A body that is not a JSON object, or members that
// are not strings, yield "".
func Parse(body []byte) (code, description string) {
	var fields map[string]any
	if json.Unmarshal(body, &fields) != nil {
		return "", ""
	}
	code, _ = fields["error"].(string)
	description, _ = fields["error_description"].(string)
	return code, description
}

// AuthRequired reports whether a non-200 token endpoint response is an
// auth_required failure: an error code in the set above on any status, or any
// 401 whatever its body (Launchpad and other servers refuse with a bare
// status). Everything else is api_error.
func AuthRequired(status int, code string) bool {
	return status == 401 || authRequiredCodes[code]
}
