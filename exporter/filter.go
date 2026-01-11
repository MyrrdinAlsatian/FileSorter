package exporter

import "FileRecoveryOrganizer/scanner"

type Filter func(scanner.Result) bool

func ByType(types ...string) Filter {
	set := map[string]struct{}{}
	for _, t := range types {
		set[t] = struct{}{}
	}

	return func(res scanner.Result) bool {
		_, ok := set[res.Type]
		return ok
	}
}
