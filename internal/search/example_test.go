package search_test

import (
	"fmt"

	"github.com/blindmaster24/MgkeTimetableBot/internal/search"
)

// A typed surname narrows the list; a typo still finds the teacher.
func ExampleSearchTeachers() {
	teachers := []string{"Иванов И.И.", "Иванова А.А.", "Петров П.П."}

	for _, query := range []string{"иванов", "ивнаов"} {
		results := search.SearchTeachers(query, teachers, nil, 2)
		fmt.Println(query, "→", results[0].Name)
	}

	// Output:
	// иванов → Иванов И.И.
	// ивнаов → Иванов И.И.
}

// An alias points at the teacher the chat actually calls them.
func ExampleSearchTeachers_alias() {
	aliases := []search.Alias{{Key: "классрук", Value: "Иванова А.А."}}

	results := search.SearchTeachers("классрук", []string{"Иванов И.И.", "Иванова А.А."}, aliases, 1)
	fmt.Println(results[0].Name, results[0].ViaAlias)

	// Output:
	// Иванова А.А. классрук
}
