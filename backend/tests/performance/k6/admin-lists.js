import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 5),
  duration: __ENV.DURATION || '30s',
};

export default function () {
  const platform = http.get(`${BASE}/platformapi/auth.admin/lists?page_no=1&page_size=20`, {
    headers: { token: __ENV.PLATFORM_TOKEN || '' },
  });
  check(platform, { 'platform lists': (r) => r.status === 200 });

  const tenant = http.get(`${BASE}/tenantapi/auth.admin/lists?page_no=1&page_size=20`, {
    headers: { token: __ENV.TENANT_TOKEN || '', Host: __ENV.TENANT_HOST || '127.0.0.1' },
  });
  check(tenant, { 'tenant lists': (r) => r.status === 200 });
}
