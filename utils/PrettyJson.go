package utils

import (
	bytes2 "bytes"
	"encoding/json"
)

func PrettyJsonString(obj any) string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	var prettyJsonBuffer bytes2.Buffer
	err = json.Indent(&prettyJsonBuffer, bytes, "", "\t")
	if err != nil {
		return ""
	}
	return prettyJsonBuffer.String()
}
