// Package sqlassets embeds dumps that PHP stored under server/.
// Disk copies remain preferred so operators can customize SQL; embed is the
// fallback after the PHP tree is removed.
package sqlassets

import _ "embed"

//go:embed like.sql
var LikeSQL string

//go:embed tenant.sql
var TenantSQL string

//go:embed tenantData.sql
var TenantDataSQL string
