import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 5),
  duration: __ENV.DURATION || '30s',
};

export default function () {
  const res = http.get(`${BASE}/api/user/center`, {
    headers: { token: __ENV.USER_TOKEN || '', Host: __ENV.TENANT_HOST || '127.0.0.1' },
  });
  check(res, { 'center': (r) => r.status === 200 });
}
