package parser

import (
	"fmt"
	"os"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestProbeTeacherPage(t *testing.T) {
	f, err := os.Open(`D:\tmp\teachers.html`)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatal(err)
	}
	teachers, _ := NewTeacherParser(doc).Parse()
	for name, teacher := range teachers {
		if name != "Зеленкевич Е. А." {
			continue
		}
		for _, day := range teacher.Days {
			if day.Day == "25.11.2026" {
				for _, l := range day.Lessons {
					if l == nil {
						fmt.Println("nil")
						continue
					}
					fmt.Printf("%q type=%v group=%v cabinet=%v\n", l.Lesson, l.Type, l.Group, l.Cabinet)
				}
			}
		}
	}
}
