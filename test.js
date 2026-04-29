import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 20,
  duration: '10s',
};

export default function () {
  const res = http.get('http://localhost:8080/api/backend/1090');

  check(res, {
    'status 200': (r) => r.status === 200,
  });

  sleep(1);
}