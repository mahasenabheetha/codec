package fake

import "testing"

// FuzzSpec: any column spec either fails or makes rows in every format,
// without panicking (ranges at the edges of int64 included).
func FuzzSpec(f *testing.F) {
	for _, s := range []string{
		"id:id,name:name,age:integer:18:90",
		"a:integer:-9223372036854775808:9223372036854775807",
		"n:number:0:1:3,c:choice:a|b?50",
		"d:date:2024-01-01:2024-01-01,t:datetime",
		"users",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, spec string) {
		cols, err := ParseSpec(spec)
		if err != nil {
			return
		}
		for _, format := range Formats {
			Write(cols, 3, 1, format, "t")
		}
	})
}
