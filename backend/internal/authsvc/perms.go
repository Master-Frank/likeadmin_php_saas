package authsvc

// BtnAuth matches PHP AuthLogic::getBtnAuthByRoleId.
// Root or a role that already covers every enabled perms value returns ["*"].
func BtnAuth(root bool, rolePerms, allPerms []string) []string {
	if root {
		return []string{"*"}
	}
	role := uniqueNonEmpty(rolePerms)
	all := uniqueNonEmpty(allPerms)
	have := make(map[string]struct{}, len(role))
	for _, p := range role {
		have[p] = struct{}{}
	}
	for _, p := range all {
		if _, ok := have[p]; !ok {
			return role
		}
	}
	return []string{"*"}
}

func uniqueNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, p := range in {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
