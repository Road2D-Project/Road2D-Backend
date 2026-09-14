package mail

// Kind identifies a built-in or registered mail/page template.
type Kind string

const (
	KindForgetPassword    Kind = "forget-password"
	KindResetPasswordPage Kind = "reset-password-page"
)

type ForgetPasswordForm struct {
	Username      string
	ResetURL      string
	ExpireMinutes int
}

type ResetPasswordPageForm struct {
	Valid    bool
	Username string
	Error    string
}
