package utils

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
)

func Md5(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	cipherStr := h.Sum(nil)
	return hex.EncodeToString(cipherStr)
}

func ObjectToMd5(o interface{}) string {
	bs, err := json.Marshal(o)
	if err != nil {
		return ""
	}
	return Md5(string(bs))
}
