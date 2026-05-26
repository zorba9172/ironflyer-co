package mobile

import "strings"

// buildExpoQRPayload returns the exp://… URL the Expo Go app expects
// when scanning a QR code. The frontend renders the QR client-side
// (qrcode.react) from this payload — keeping QR encoding off the
// runtime avoids pulling a new top-level Go dep just for one PNG.
//
// If tunnelURL is present (Expo's ngrok-style relay), it is the
// canonical payload because it works across networks. Otherwise we
// fall back to the LAN URL (works only on the same Wi-Fi).
func buildExpoQRPayload(tunnelURL, lanURL string) string {
	if u := strings.TrimSpace(tunnelURL); u != "" {
		return u
	}
	return strings.TrimSpace(lanURL)
}
