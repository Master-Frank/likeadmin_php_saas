// Package sqlassets embeds install and tenant SQL dumps.
package sqlassets

import _ "embed"

//go:embed like.sql
var LikeSQL string

//go:embed tenant.sql
var TenantSQL string

//go:embed tenantData.sql
var TenantDataSQL string
