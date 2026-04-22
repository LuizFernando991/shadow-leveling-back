package email

import (
	"bytes"
	"fmt"
	"html/template"
)

var verificationCodeTmpl = template.Must(template.New("verification_code").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Verification Code</title>
</head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:40px 0;">
    <tr>
      <td align="center">
        <table width="560" cellpadding="0" cellspacing="0"
               style="background:#ffffff;border-radius:8px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.1);">
          <tr>
            <td style="background:#111827;padding:28px 40px;">
              <h1 style="margin:0;color:#ffffff;font-size:20px;font-weight:700;letter-spacing:.5px;">Gym</h1>
            </td>
          </tr>
          <tr>
            <td style="padding:40px;">
              <h2 style="margin:0 0 8px;color:#111827;font-size:20px;font-weight:600;">
                Verification code
              </h2>
              <p style="margin:0 0 32px;color:#6b7280;font-size:15px;line-height:1.6;">
                Use the code below to complete your request.
                It expires in <strong style="color:#374151;">15 minutes</strong>.
              </p>
              <div style="background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;
                          padding:28px 24px;text-align:center;margin-bottom:32px;">
                <span style="font-size:40px;font-weight:700;letter-spacing:14px;
                             color:#111827;font-variant-numeric:tabular-nums;">
                  {{.Code}}
                </span>
              </div>
              <p style="margin:0;color:#9ca3af;font-size:13px;line-height:1.6;">
                If you didn't request this code, you can safely ignore this email.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background:#f9fafb;padding:16px 40px;border-top:1px solid #e5e7eb;">
              <p style="margin:0;color:#9ca3af;font-size:12px;">
                &copy; Gym. All rights reserved.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

// VerificationCodeHTML renders the verification-code email template and returns
// the HTML string ready to be sent.
func VerificationCodeHTML(code string) (string, error) {
	var buf bytes.Buffer
	if err := verificationCodeTmpl.Execute(&buf, struct{ Code string }{Code: code}); err != nil {
		return "", fmt.Errorf("email: render verification template: %w", err)
	}
	return buf.String(), nil
}
