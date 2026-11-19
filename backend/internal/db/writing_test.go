package db

import (
	"errors"
	"testing"
)

func TestWritingPiecesAreScopedAndCascade(t *testing.T) {
	d := open(t)
	a, _ := d.CreateUser("a@example.com", "h", RoleAdmin)
	b, _ := d.CreateUser("b@example.com", "h", RoleLearner)

	p, err := d.CreateWritingPiece(a, "t1", "Cím", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpdateWritingPiece(a, p.ID, "Új", "Szia!"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.WritingPiece(b, p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another user's piece: %v, want ErrNotFound", err)
	}
	if _, err := d.UpdateWritingPiece(b, p.ID, "x", "y"); !errors.Is(err, ErrNotFound) {
		t.Errorf("another user's update: %v", err)
	}
	if _, err := d.AddWritingCheck(b, WritingCheck{PieceID: p.ID, Source: "external", Model: "m", Lang: "ru", Raw: "r"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("another user's check: %v", err)
	}

	c, err := d.AddWritingCheck(a, WritingCheck{PieceID: p.ID, Source: "local", Model: "m", Lang: "ru",
		PromptVersion: 1, CheckedText: "Szia!", Raw: "raw", Interrupted: true})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Interrupted || c.Raw != "raw" || c.CreatedAt.IsZero() {
		t.Errorf("stored check = %+v", c)
	}
	rows, err := d.WritingPieces(a)
	if err != nil || len(rows) != 1 || rows[0].Checks != 1 || rows[0].LastCheck == nil || rows[0].LastCheck.Model != "m" || rows[0].Title != "Új" {
		t.Fatalf("list = %+v %v", rows, err)
	}
	if rows, _ := d.WritingPieces(b); len(rows) != 0 {
		t.Errorf("b sees %d pieces", len(rows))
	}
	if err := d.DeleteWritingCheck(b, p.ID, c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("other user's check delete: %v", err)
	}
	if err := d.DeleteWritingPiece(a, p.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM writing_check`).Scan(&n)
	if n != 0 {
		t.Errorf("checks survived their piece: %d", n)
	}
}

func TestWritingCheckCap(t *testing.T) {
	d := open(t)
	u, _ := d.CreateUser("a@example.com", "h", RoleAdmin)
	p, _ := d.CreateWritingPiece(u, "", "", "A2")
	for i := 0; i < MaxChecksPerPiece; i++ {
		if _, err := d.AddWritingCheck(u, WritingCheck{PieceID: p.ID, Source: "external", Model: "m", Lang: "en", Raw: "r"}); err != nil {
			t.Fatalf("check %d: %v", i, err)
		}
	}
	if _, err := d.AddWritingCheck(u, WritingCheck{PieceID: p.ID, Source: "external", Model: "m", Lang: "en", Raw: "r"}); !errors.Is(err, ErrTooManyChecks) {
		t.Errorf("over the cap: %v", err)
	}
}
