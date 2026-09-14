package mail

import (
	"strings"
	"testing"
)

func TestRenderForgetPasswordMailEmbedsResetURL(t *testing.T) {
	html, err := Render(KindForgetPassword, ForgetPasswordForm{
		Username:      "traveler",
		ResetURL:      "http://localhost:8080/v1/auth/user/reset-password/test-token",
		ExpireMinutes: 15,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `href="http://localhost:8080/v1/auth/user/reset-password/test-token"`) {
		t.Fatalf("reset href missing or escaped: %s", html)
	}
}

func TestRenderResetPasswordPageInvalidToken(t *testing.T) {
	html, err := Render(KindResetPasswordPage, ResetPasswordPageForm{
		Valid: false,
		Error: "Token đã hết hạn hoặc không hợp lệ.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Liên kết không còn hiệu lực") {
		t.Fatalf("invalid page missing copy: %s", html)
	}
}

func TestRenderHTMLFormUsesCallerTemplate(t *testing.T) {
	html, err := RenderHTMLForm(`<p>Hi {{.Name}}</p>`, map[string]string{"Name": "An"})
	if err != nil {
		t.Fatal(err)
	}
	if html != "<p>Hi An</p>" {
		t.Fatalf("got %q", html)
	}
}

func TestRegisterCustomKind(t *testing.T) {
	kind := Kind("test-custom-kind")
	if err := Register(kind, "Hello", `<p>{{.Title}}</p>`); err != nil {
		t.Fatal(err)
	}
	html, err := Render(kind, struct{ Title string }{Title: "Trip invite"})
	if err != nil {
		t.Fatal(err)
	}
	if html != "<p>Trip invite</p>" {
		t.Fatalf("got %q", html)
	}
}

func TestSendRejectsPageKind(t *testing.T) {
	err := NewFromEnv().Send(t.Context(), KindResetPasswordPage, "a@b.com", ResetPasswordPageForm{})
	if err == nil || !strings.Contains(err.Error(), "page template") {
		t.Fatalf("expected page template error, got %v", err)
	}
}
