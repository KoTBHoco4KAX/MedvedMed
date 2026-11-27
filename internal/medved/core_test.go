package medved

import "testing"

func TestCoreFirstNameStaysEmpty(t *testing.T) {
	var user coreUser
	if got := user.firstName(); got != "" {
		t.Fatalf("пустое имя из Core должно остаться пустым, получено %q", got)
	}

	user.FullName.FirstName = "  "
	user.FullNameCamel.FirstName = "  "
	if got := user.firstName(); got != "" {
		t.Fatalf("пробелы должны превращаться в пустую строку, получено %q", got)
	}

	user.FullNameCamel.FirstName = "Иван"
	if got := user.firstName(); got != "Иван" {
		t.Fatalf("ожидалось имя из fullName, получено %q", got)
	}
}
