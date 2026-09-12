import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8080';
const HOST = __ENV.TENANT_HOST || '127.0.0.1';

export const options = {
  vus: Number(__ENV.VUS || 5),
  duration: __ENV.DURATION || '30s',
};

export default function () {
  const headers = { Host: HOST };
  check(http.get(`${BASE}/api/article/lists?page_no=1&page_size=20`, { headers }), {
    'lists': (r) => r.status === 200,
  });
  check(http.get(`${BASE}/api/article/cate`, { headers }), {
    'cate': (r) => r.status === 200,
  });
  check(http.get(`${BASE}/api/article/lists?keyword=test&page_no=1&page_size=20`, { headers }), {
    'keyword': (r) => r.status === 200,
  });
}
