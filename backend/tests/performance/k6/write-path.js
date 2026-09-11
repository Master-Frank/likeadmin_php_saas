import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 2),
  duration: __ENV.DURATION || '20s',
};

export default function () {
  // Login / SMS / pay must use sandbox credentials. These requests exist to
  // drive the write path; they are not a pass/fail product test.
  const login = http.post(`${BASE}/api/login/account`, JSON.stringify({
    account: __ENV.USER_ACCOUNT || 'demo',
    password: __ENV.USER_PASSWORD || 'invalid',
    terminal: 1,
  }), {
    headers: {
      'Content-Type': 'application/json',
      Host: __ENV.TENANT_HOST || '127.0.0.1',
    },
  });
  check(login, { 'login http': (r) => r.status === 200 });
}
