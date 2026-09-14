package mail

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"sync"
)

//go:embed templates/mail/*.html templates/page/*.html
var builtinFS embed.FS

type spec struct {
	subject string
	tpl     *template.Template
	page    bool
}

var (
	catalogMu sync.RWMutex
	catalog   = map[Kind]*spec{}
)

func init() {
	mustRegisterFile(KindForgetPassword, "Đặt lại mật khẩu Road2D", "templates/mail/forget-password.html", false)
	mustRegisterFile(KindResetPasswordPage, "", "templates/page/reset-password.html", true)
}

func mustRegisterFile(kind Kind, subject, path string, page bool) {
	tpl := template.Must(template.ParseFS(builtinFS, path))
	catalog[kind] = &spec{subject: subject, tpl: tpl, page: page}
}

func Register(kind Kind, subject, htmlTemplate string) error {
	return registerParsed(kind, subject, htmlTemplate, false)
}

func RegisterPage(kind Kind, htmlTemplate string) error {
	return registerParsed(kind, "", htmlTemplate, true)
}

func registerParsed(kind Kind, subject, htmlTemplate string, page bool) error {
	if kind == "" {
		return fmt.Errorf("mail kind is required")
	}
	tpl, err := template.New(string(kind)).Parse(htmlTemplate)
	if err != nil {
		return err
	}
	catalogMu.Lock()
	defer catalogMu.Unlock()
	catalog[kind] = &spec{subject: subject, tpl: tpl, page: page}
	return nil
}

func Render(kind Kind, form any) (string, error) {
	catalogMu.RLock()
	item, ok := catalog[kind]
	catalogMu.RUnlock()
	if !ok {
		return "", fmt.Errorf("unknown mail kind %q", kind)
	}
	return execute(item.tpl, templateData(form))
}

func RenderHTMLForm(htmlTemplate string, form any) (string, error) {
	tpl, err := template.New("inline").Parse(htmlTemplate)
	if err != nil {
		return "", err
	}
	return execute(tpl, form)
}

func RenderFile(fsys fs.FS, name string, form any) (string, error) {
	tpl, err := template.ParseFS(fsys, name)
	if err != nil {
		return "", err
	}
	return execute(tpl, form)
}

func execute(tpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func lookup(kind Kind) (*spec, error) {
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	item, ok := catalog[kind]
	if !ok {
		return nil, fmt.Errorf("unknown mail kind %q", kind)
	}
	copied := *item
	return &copied, nil
}

type forgetPasswordView struct {
	Username      string
	ResetURL      template.URL
	ExpireMinutes int
}

func templateData(form any) any {
	switch typed := form.(type) {
	case ForgetPasswordForm:
		return forgetPasswordView{
			Username:      typed.Username,
			ResetURL:      template.URL(typed.ResetURL),
			ExpireMinutes: typed.ExpireMinutes,
		}
	case *ForgetPasswordForm:
		if typed == nil {
			return form
		}
		return templateData(*typed)
	default:
		return form
	}
}
