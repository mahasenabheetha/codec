package fake

import (
	"fmt"
	"strconv"
	"strings"
)

// Presets are column sets for the usual test tables.
var Presets = map[string][]Column{
	"users": {
		{Name: "id", Kind: "id"},
		{Name: "name", Kind: "name"},
		{Name: "email", Kind: "email"},
		{Name: "city", Kind: "city"},
		{Name: "active", Kind: "boolean"},
		{Name: "created_at", Kind: "datetime"},
	},
	"orders": {
		{Name: "order_id", Kind: "uuid"},
		{Name: "customer", Kind: "name"},
		{Name: "amount", Kind: "number", Min: "5", Max: "500"},
		{Name: "currency", Kind: "choice", Choices: []string{"EUR", "USD", "SEK", "LKR"}},
		{Name: "status", Kind: "choice", Choices: []string{"pending", "paid", "shipped", "refunded"}},
		{Name: "ordered_on", Kind: "date"},
	},
	"servers": {
		{Name: "host", Kind: "word"},
		{Name: "ip", Kind: "ipv4"},
		{Name: "ipv6", Kind: "ipv6"},
		{Name: "region", Kind: "choice", Choices: []string{"eu-north", "eu-west", "us-east", "ap-south"}},
		{Name: "cpus", Kind: "integer", Min: "2", Max: "64"},
		{Name: "healthy", Kind: "boolean"},
	},
}

// ParseSpec reads columns written as name:kind[:arg…], comma-separated:
// integer, number, date and datetime take min and max (number a third,
// decimals); choice takes its choices split by |; a trailing ?N leaves
// N% of a column blank. A preset name stands for its columns.
//
//	id:id,name:name,age:integer:18:90,plan:choice:free|pro,note:sentence?20
func ParseSpec(spec string) ([]Column, error) {
	spec = strings.TrimSpace(spec)
	if p, ok := Presets[spec]; ok {
		return p, nil
	}
	var cols []Column
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		c := Column{}
		if i := strings.LastIndex(part, "?"); i > 0 {
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n < 0 || n > 100 {
				return nil, fmt.Errorf("%q: the blank share after ? is 0 to 100", part)
			}
			c.Blank, part = n, part[:i]
		}
		f := strings.Split(part, ":")
		if len(f) < 2 {
			return nil, fmt.Errorf("%q: write a column as name:kind", part)
		}
		c.Name, c.Kind = f[0], f[1]
		args := f[2:]
		switch c.Kind {
		case "integer", "number", "date", "datetime":
			if len(args) > 0 {
				c.Min = args[0]
			}
			if len(args) > 1 {
				c.Max = args[1]
			}
			if c.Kind == "number" && len(args) > 2 {
				d, err := strconv.Atoi(args[2])
				if err != nil {
					return nil, fmt.Errorf("%q: decimals is a whole number", part)
				}
				c.Decimals = d
			}
		case "choice":
			if len(args) > 0 {
				c.Choices = strings.Split(strings.Join(args, ":"), "|")
			}
		default:
			if len(args) > 0 {
				return nil, fmt.Errorf("%q: %s takes no arguments", part, c.Kind)
			}
		}
		cols = append(cols, c)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns: give name:kind pairs or a preset (users, orders, servers)")
	}
	return cols, nil
}
