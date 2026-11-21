package rooms_test

import (
	"fmt"

	"github.com/blindmaster24/MgkeTimetableBot/internal/rooms"
)

// The site spells the same room in several ways; Key folds them into one.
func ExampleKey() {
	for _, room := range []string{"Кн 1-109(к)", "каб. Кн 1-109", "КН 1-109"} {
		fmt.Println(rooms.Key(room))
	}

	// Output:
	// кн1-109
	// кн1-109
	// кн1-109
}

// Universe keeps one label per room, and Free lists the ones nobody uses.
func ExampleFree() {
	universe := rooms.Universe([]string{"Кн 1-109(к)", "каб. Кн 1-109", "Кн 2-201"})
	occupied := map[string]bool{rooms.Key("Кн 1-109"): true}

	fmt.Println(rooms.Free(universe, occupied))

	// Output:
	// [Кн 2-201]
}
