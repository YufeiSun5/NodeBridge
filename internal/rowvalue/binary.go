package rowvalue

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const binaryTag = "$nodebridge_binary_base64"

// Binary retains bytes through JSON persistence, broker transport and SQL binding.
type Binary []byte

func (b Binary) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{binaryTag: base64.StdEncoding.EncodeToString(b)})
}

func (b Binary) Value() (driver.Value, error) {
	return []byte(b), nil
}

// Canal encodes BINARY/VARBINARY/BLOB bytes as ISO-8859-1 code points.
func FromCanal(value string) (Binary, error) {
	result := make(Binary, 0, len(value))
	for _, r := range value {
		if r > 255 {
			return nil, fmt.Errorf("invalid canal binary code point U+%04X", r)
		}
		result = append(result, byte(r))
	}
	return result, nil
}

func Restore(rows ...map[string]any) error {
	for _, row := range rows {
		for column, value := range row {
			object, ok := value.(map[string]any)
			if !ok {
				continue
			}
			encoded, tagged := object[binaryTag]
			if !tagged {
				continue
			}
			text, ok := encoded.(string)
			if !ok || len(object) != 1 {
				return fmt.Errorf("invalid binary envelope for column %s", column)
			}
			bytes, err := base64.StdEncoding.Strict().DecodeString(text)
			if err != nil {
				return fmt.Errorf("invalid binary base64 for column %s: %w", column, err)
			}
			row[column] = Binary(bytes)
		}
	}
	return nil
}
