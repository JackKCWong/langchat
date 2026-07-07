package chat

import "encoding/base64"

func decodeBase64Std(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func decodeBase64RawStd(s string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(s)
}