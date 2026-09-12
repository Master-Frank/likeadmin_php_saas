import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: 1,
  duration: __ENV.DURATION || '30s',
};

export default function () {
  const create = http.get(`${BASE}/platformapi/setting.system.log/lists?export=2&page_start=1&page_end=1&page_size=25`, {
    headers: { token: __ENV.PLATFORM_TOKEN || '' },
  });
  check(create, { 'export accepted': (r) => r.status === 200 });
  sleep(1);
}
