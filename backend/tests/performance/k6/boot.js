import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  scenarios: {
    boot: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RATE || 20),
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: 10,
    },
  },
};

export default function () {
  const res = http.get(`${BASE}/api/index/config`, {
    headers: { Host: __ENV.TENANT_HOST || '127.0.0.1' },
  });
  check(res, { 'config 200': (r) => r.status === 200 });
  sleep(0.01);
}
