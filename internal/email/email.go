package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"
)

// Service sends transactional email via the Resend API.
type Service struct {
	apiKey    string
	fromEmail string
	baseURL   string
}

func NewService(apiKey, fromEmail, baseURL string) *Service {
	return &Service{apiKey: apiKey, fromEmail: fromEmail, baseURL: baseURL}
}

// Enabled reports whether email sending is configured.
func (s *Service) Enabled() bool {
	return s.apiKey != ""
}

// SendPasswordReset emails a 24-hour password-reset link to the user.
// resetToken is the raw token ID (not the full URL).
// userRole drives which getting-started guide URL is included.
func (s *Service) SendPasswordReset(toEmail, toName, resetToken, userRole string) error {
	resetLink := fmt.Sprintf("%s/reset-password/%s", s.baseURL, resetToken)
	guideURL := fmt.Sprintf("%s/guide/%s", s.baseURL, roleSlug(userRole))

	html, err := renderEmail(toName, resetLink, guideURL, roleLabel(userRole))
	if err != nil {
		return fmt.Errorf("email: render: %w", err)
	}
	return s.send(toEmail, "You've been added to IPN Events", html)
}

// SendInvite emails an invitation link to a new user.
func (s *Service) SendInvite(toEmail, inviteToken, role string) error {
	inviteLink := fmt.Sprintf("%s/invite/%s", s.baseURL, inviteToken)
	guideURL := fmt.Sprintf("%s/guide/%s", s.baseURL, roleSlug(role))

	html, err := renderInviteEmail(toEmail, inviteLink, guideURL, roleLabel(role))
	if err != nil {
		return fmt.Errorf("email: render invite: %w", err)
	}
	return s.send(toEmail, "You're invited to IPN Events", html)
}

// SendEventNotification emails a team member about an event update (approval, rejection, or comment).
func (s *Service) SendEventNotification(toEmail, toName, eventName, eventID, action, comment string) error {
	if !s.Enabled() {
		return nil
	}
	eventLink := fmt.Sprintf("%s/events/%s", s.baseURL, eventID)

	subject := "IPN Events — "
	switch action {
	case "approved":
		subject += "Your event has been approved"
	case "rejected":
		subject += "Changes requested for your event"
	case "comment":
		subject += "New comment on your event"
	default:
		subject += "Event update"
	}

	html, err := renderEventNotificationEmail(toName, eventName, eventLink, action, comment)
	if err != nil {
		return fmt.Errorf("email: render event notification: %w", err)
	}
	return s.send(toEmail, subject, html)
}

func (s *Service) send(to, subject, htmlBody string) error {
	payload := map[string]interface{}{
		"from":    s.fromEmail,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("resend: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func roleSlug(role string) string {
	switch role {
	case "admin":
		return "admin"
	case "viewer":
		return "viewer"
	default:
		return "member"
	}
}

func roleLabel(role string) string {
	switch role {
	case "admin":
		return "Admin"
	case "viewer":
		return "Viewer"
	default:
		return "Team Member"
	}
}

// ── Email HTML template ────────────────────────────────────────────────────

const emailTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1.0">
  <title>IPN Events — Set your password</title>
</head>
<body style="margin:0;padding:0;background:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="background:#f1f5f9;padding:48px 16px;">
    <tr><td align="center">

      <table width="560" cellpadding="0" cellspacing="0" role="presentation"
             style="max-width:560px;width:100%;background:#ffffff;border-radius:16px;border:1px solid #e2e8f0;overflow:hidden;">

        <!-- Header -->
        <tr>
          <td style="background:#0a1628;padding:28px 40px;text-align:center;">
            <p style="margin:0;font-size:11px;font-weight:600;letter-spacing:0.15em;text-transform:uppercase;color:#93c5fd;">IPN Southeast</p>
            <h1 style="margin:6px 0 0;font-size:22px;font-weight:700;color:#ffffff;">Events Portal</h1>
          </td>
        </tr>

        <!-- Body -->
        <tr>
          <td style="padding:40px 40px 32px;">

            <h2 style="margin:0 0 8px;font-size:20px;font-weight:600;color:#0f172a;">Hi {{.Name}},</h2>
            <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#475569;">
              You've been added to the <strong style="color:#0f172a;">IPN Southeast Events Portal</strong>
              as a <strong style="color:#0f172a;">{{.RoleLabel}}</strong>.
              Click below to set your password and sign in.
            </p>

            <!-- Primary CTA -->
            <table cellpadding="0" cellspacing="0" role="presentation" style="margin:0 0 36px;">
              <tr>
                <td style="background:#1d4ed8;border-radius:10px;box-shadow:0 1px 3px rgba(0,0,0,0.2);">
                  <a href="{{.ResetLink}}"
                     style="display:inline-block;padding:14px 36px;font-size:16px;font-weight:600;color:#ffffff;text-decoration:none;letter-spacing:0.01em;">
                    Set Password &amp; Sign In &#8594;
                  </a>
                </td>
              </tr>
            </table>

            <!-- Divider -->
            <hr style="border:none;border-top:1px solid #e2e8f0;margin:0 0 28px;">

            <!-- Getting started -->
            <h3 style="margin:0 0 10px;font-size:14px;font-weight:600;color:#0f172a;">&#127891; Getting started</h3>
            <p style="margin:0 0 18px;font-size:14px;line-height:1.65;color:#64748b;">
              We've put together a quick guide showing you exactly what you can do as a
              <strong>{{.RoleLabel}}</strong> in the portal.
            </p>
            <a href="{{.GuideURL}}"
               style="display:inline-block;padding:10px 22px;font-size:14px;font-weight:500;color:#1d4ed8;background:#eff6ff;border:1px solid #bfdbfe;border-radius:8px;text-decoration:none;">
              View Getting Started Guide &#8594;
            </a>

          </td>
        </tr>

        <!-- Footer -->
        <tr>
          <td style="background:#f8fafc;border-top:1px solid #e2e8f0;padding:20px 40px;">
            <p style="margin:0 0 8px;font-size:12px;line-height:1.6;color:#94a3b8;">
              This link expires in <strong>24 hours</strong>.
              If you weren't expecting this email you can safely ignore it.
            </p>
            <p style="margin:0;font-size:11px;color:#cbd5e1;word-break:break-all;">
              If the button doesn't work, copy this into your browser:<br>
              {{.ResetLink}}
            </p>
          </td>
        </tr>

      </table>

    </td></tr>
  </table>
</body>
</html>`

func renderEmail(name, resetLink, guideURL, rl string) (string, error) {
	t, err := template.New("email").Parse(emailTmpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	err = t.Execute(&buf, map[string]string{
		"Name":      name,
		"ResetLink": resetLink,
		"GuideURL":  guideURL,
		"RoleLabel": rl,
	})
	return buf.String(), err
}

// ── Invite email template ──────────────────────────────────────────────────

const inviteEmailTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1.0">
  <title>IPN Events — You're Invited</title>
</head>
<body style="margin:0;padding:0;background:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="background:#f1f5f9;padding:48px 16px;">
    <tr><td align="center">

      <table width="560" cellpadding="0" cellspacing="0" role="presentation"
             style="max-width:560px;width:100%;background:#ffffff;border-radius:16px;border:1px solid #e2e8f0;overflow:hidden;">

        <!-- Header -->
        <tr>
          <td style="background:#0a1628;padding:28px 40px;text-align:center;">
            <p style="margin:0;font-size:11px;font-weight:600;letter-spacing:0.15em;text-transform:uppercase;color:#93c5fd;">IPN Southeast</p>
            <h1 style="margin:6px 0 0;font-size:22px;font-weight:700;color:#ffffff;">Events Portal</h1>
          </td>
        </tr>

        <!-- Body -->
        <tr>
          <td style="padding:40px 40px 32px;">

            <h2 style="margin:0 0 8px;font-size:20px;font-weight:600;color:#0f172a;">You're Invited!</h2>
            <p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#475569;">
              You've been invited to join the <strong style="color:#0f172a;">IPN Southeast Events Portal</strong>
              as a <strong style="color:#0f172a;">{{.RoleLabel}}</strong>.
              Click below to create your account.
            </p>

            <!-- Primary CTA -->
            <table cellpadding="0" cellspacing="0" role="presentation" style="margin:0 0 36px;">
              <tr>
                <td style="background:#1d4ed8;border-radius:10px;box-shadow:0 1px 3px rgba(0,0,0,0.2);">
                  <a href="{{.InviteLink}}"
                     style="display:inline-block;padding:14px 36px;font-size:16px;font-weight:600;color:#ffffff;text-decoration:none;letter-spacing:0.01em;">
                    Accept Invite &amp; Sign Up &#8594;
                  </a>
                </td>
              </tr>
            </table>

            <!-- Divider -->
            <hr style="border:none;border-top:1px solid #e2e8f0;margin:0 0 28px;">

            <!-- Getting started -->
            <h3 style="margin:0 0 10px;font-size:14px;font-weight:600;color:#0f172a;">&#127891; Getting started</h3>
            <p style="margin:0 0 18px;font-size:14px;line-height:1.65;color:#64748b;">
              Check out our quick guide to see what you can do as a
              <strong>{{.RoleLabel}}</strong>.
            </p>
            <a href="{{.GuideURL}}"
               style="display:inline-block;padding:10px 22px;font-size:14px;font-weight:500;color:#1d4ed8;background:#eff6ff;border:1px solid #bfdbfe;border-radius:8px;text-decoration:none;">
              View Getting Started Guide &#8594;
            </a>

          </td>
        </tr>

        <!-- Footer -->
        <tr>
          <td style="background:#f8fafc;border-top:1px solid #e2e8f0;padding:20px 40px;">
            <p style="margin:0 0 8px;font-size:12px;line-height:1.6;color:#94a3b8;">
              This invite expires in <strong>7 days</strong>.
              If you weren't expecting this email you can safely ignore it.
            </p>
            <p style="margin:0;font-size:11px;color:#cbd5e1;word-break:break-all;">
              If the button doesn't work, copy this into your browser:<br>
              {{.InviteLink}}
            </p>
          </td>
        </tr>

      </table>

    </td></tr>
  </table>
</body>
</html>`

func renderInviteEmail(email, inviteLink, guideURL, roleLabel string) (string, error) {
	t, err := template.New("invite").Parse(inviteEmailTmpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	err = t.Execute(&buf, map[string]string{
		"Email":      email,
		"InviteLink": inviteLink,
		"GuideURL":   guideURL,
		"RoleLabel":  roleLabel,
	})
	return buf.String(), err
}

// ── Event notification email template ─────────────────────────────────────

const eventNotificationTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1.0">
  <title>IPN Events — Event Update</title>
</head>
<body style="margin:0;padding:0;background:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="background:#f1f5f9;padding:48px 16px;">
    <tr><td align="center">

      <table width="560" cellpadding="0" cellspacing="0" role="presentation"
             style="max-width:560px;width:100%;background:#ffffff;border-radius:16px;border:1px solid #e2e8f0;overflow:hidden;">

        <!-- Header -->
        <tr>
          <td style="background:#0a1628;padding:28px 40px;text-align:center;">
            <p style="margin:0;font-size:11px;font-weight:600;letter-spacing:0.15em;text-transform:uppercase;color:#93c5fd;">IPN Southeast</p>
            <h1 style="margin:6px 0 0;font-size:22px;font-weight:700;color:#ffffff;">Events Portal</h1>
          </td>
        </tr>

        <!-- Body -->
        <tr>
          <td style="padding:40px 40px 32px;">

            <h2 style="margin:0 0 8px;font-size:20px;font-weight:600;color:#0f172a;">Hi {{.Name}},</h2>

            {{if eq .Action "approved"}}
            <p style="margin:0 0 16px;font-size:15px;line-height:1.65;color:#475569;">
              Great news! Your event <strong style="color:#0f172a;">{{.EventName}}</strong> has been <span style="color:#16a34a;font-weight:600;">approved</span>.
            </p>
            {{else if eq .Action "rejected"}}
            <p style="margin:0 0 16px;font-size:15px;line-height:1.65;color:#475569;">
              Your event <strong style="color:#0f172a;">{{.EventName}}</strong> needs some changes before it can be approved.
            </p>
            {{else}}
            <p style="margin:0 0 16px;font-size:15px;line-height:1.65;color:#475569;">
              A new comment has been added to your event <strong style="color:#0f172a;">{{.EventName}}</strong>.
            </p>
            {{end}}

            {{if .Comment}}
            <div style="margin:0 0 24px;padding:16px;background:#f8fafc;border-left:4px solid {{if eq .Action "approved"}}#16a34a{{else if eq .Action "rejected"}}#dc2626{{else}}#2563eb{{end}};border-radius:0 8px 8px 0;">
              <p style="margin:0;font-size:14px;line-height:1.65;color:#334155;white-space:pre-wrap;">{{.Comment}}</p>
            </div>
            {{end}}

            <!-- CTA -->
            <table cellpadding="0" cellspacing="0" role="presentation" style="margin:0 0 16px;">
              <tr>
                <td style="background:#1d4ed8;border-radius:10px;box-shadow:0 1px 3px rgba(0,0,0,0.2);">
                  <a href="{{.EventLink}}"
                     style="display:inline-block;padding:14px 36px;font-size:16px;font-weight:600;color:#ffffff;text-decoration:none;letter-spacing:0.01em;">
                    View Event &#8594;
                  </a>
                </td>
              </tr>
            </table>

          </td>
        </tr>

        <!-- Footer -->
        <tr>
          <td style="background:#f8fafc;border-top:1px solid #e2e8f0;padding:20px 40px;">
            <p style="margin:0;font-size:11px;color:#cbd5e1;word-break:break-all;">
              If the button doesn't work, copy this into your browser:<br>
              {{.EventLink}}
            </p>
          </td>
        </tr>

      </table>

    </td></tr>
  </table>
</body>
</html>`

func renderEventNotificationEmail(name, eventName, eventLink, action, comment string) (string, error) {
	t, err := template.New("event_notification").Parse(eventNotificationTmpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	err = t.Execute(&buf, map[string]string{
		"Name":      name,
		"EventName": eventName,
		"EventLink": eventLink,
		"Action":    action,
		"Comment":   comment,
	})
	return buf.String(), err
}
