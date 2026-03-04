package client

import "encoding/json"

type Field struct {
	Label  string
	Value  string
	Secret bool
}

func ParseFields(secretType, value string) []Field {
	switch secretType {
	case "username_password":
		var obj struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if json.Unmarshal([]byte(value), &obj) == nil {
			return []Field{
				{Label: "Username", Value: obj.Username, Secret: false},
				{Label: "Password", Value: obj.Password, Secret: true},
			}
		}
	case "key_value_pair":
		var obj map[string]string
		if json.Unmarshal([]byte(value), &obj) == nil {
			for k, v := range obj {
				return []Field{
					{Label: "Key", Value: k, Secret: false},
					{Label: "Value", Value: v, Secret: true},
				}
			}
		}
	case "tls_bundle":
		var obj struct {
			Certificate string `json:"certificate"`
			PrivateKey  string `json:"private_key"`
			CAChain     string `json:"ca_chain"`
		}
		if json.Unmarshal([]byte(value), &obj) == nil {
			return []Field{
				{Label: "Certificate", Value: obj.Certificate, Secret: false},
				{Label: "Private Key", Value: obj.PrivateKey, Secret: true},
				{Label: "CA Chain", Value: obj.CAChain, Secret: false},
			}
		}
	}
	return []Field{{Label: "Value", Value: value, Secret: true}}
}
