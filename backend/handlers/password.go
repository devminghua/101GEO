package handlers

// validatePassword 密码强度校验：≥8 位，且同时包含字母与数字。
// 返回空串表示通过，否则返回错误提示。
func validatePassword(pw string) string {
	if len(pw) < 8 {
		return "密码长度至少 8 位"
	}
	hasLetter, hasDigit := false, false
	for _, r := range pw {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return "密码须同时包含字母和数字"
	}
	return ""
}
