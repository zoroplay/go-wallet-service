package common

import (
	"testing"
)

func TestGenerateTrxNo(t *testing.T) {
	trx := GenerateTrxNo()
	if len(trx) != 7 {
		t.Errorf("Expected length 7, got %d", len(trx))
	}

	// Check if it contains valid characters
	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, char := range trx {
		isValid := false
		for _, validChar := range validChars {
			if char == validChar {
				isValid = true
				break
			}
		}
		if !isValid {
			t.Errorf("Invalid character found: %c", char)
		}
	}
}

func TestPaginateResponse(t *testing.T) {
	// Test case 1: Normal pagination
	total := int64(100)
	page := 1
	limit := 10
	data := []string{"item1", "item2"}

	res := PaginateResponse(data, total, page, limit, "")

	if res.CurrentPage != 1 {
		t.Errorf("Expected CurrentPage 1, got %d", res.CurrentPage)
	}
	if res.LastPage != 10 {
		t.Errorf("Expected LastPage 10, got %d", res.LastPage)
	}
	if res.NextPage != 2 {
		t.Errorf("Expected NextPage 2, got %d", res.NextPage)
	}
	if res.PrevPage != 0 {
		t.Errorf("Expected PrevPage 0, got %d", res.PrevPage)
	}
	if res.Count != 100 {
		t.Errorf("Expected Count 100, got %d", res.Count)
	}

	// Test case 2: Last page
	page = 10
	res = PaginateResponse(data, total, page, limit, "")
	if res.NextPage != 0 {
		t.Errorf("Expected NextPage 0 for last page, got %d", res.NextPage)
	}

	// Test case 3: Middle page
	page = 5
	res = PaginateResponse(data, total, page, limit, "")
	if res.PrevPage != 4 {
		t.Errorf("Expected PrevPage 4, got %d", res.PrevPage)
	}
	if res.NextPage != 6 {
		t.Errorf("Expected NextPage 6, got %d", res.NextPage)
	}
}
