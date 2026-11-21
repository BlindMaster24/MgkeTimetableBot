package preflight

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blindmaster24/MgkeTimetableBot/internal/utils"
)

// pageDates must cover the current week, otherwise checkGroups warns that the
// site has not published it yet.
func pageDates() (string, string) {
	today := utils.SiteNow()
	return today.Format("02.01.2006"), today.AddDate(0, 0, 1).Format("02.01.2006")
}

func groupPage() string {
	first, second := pageDates()
	return `<html><body><h1>Группа - 63ТП</h1><table>` +
		`<tr><th rowspan="2">№</th><th colspan="2">Понедельник, ` + first + `</th><th colspan="2">Вторник, ` + second + `</th></tr>` +
		`<tr><th>1</th><td>Математика</td><td>2</td><td>Физика</td></tr>` +
		`</table></body></html>`
}

func teacherPage() string {
	first, _ := pageDates()
	return `<html><body><h1>Преподаватель - Иванов И.И.</h1><table>` +
		`<tr><th rowspan="2">№</th><th colspan="2">Понедельник, ` + first + `</th></tr>` +
		`<tr><th>1</th><td>63ТП</td><td>Математика</td></tr>` +
		`</table></body></html>`
}

func pageServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/groups"):
			_, _ = w.Write([]byte(groupPage()))
		case strings.HasPrefix(r.URL.Path, "/teachers"):
			_, _ = w.Write([]byte(teacherPage()))
		default:
			http.NotFound(w, r)
		}
	}))
}
