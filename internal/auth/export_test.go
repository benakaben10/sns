package auth

// NewHMACVerifierForTest exposes HMAC verifier creation for use in external test packages.
func NewHMACVerifierForTest(secret string) Verifier {
	return NewHMACVerifier(secret, verifyConfig{})
}
