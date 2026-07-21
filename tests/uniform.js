import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 20,
  duration: '60s',
};

const firstKey = 1;
const keyCount = 10;

export default function () {
  const id = firstKey + Math.floor(Math.random() * keyCount);

  const res = http.get(`http://localhost:8080/api/backend/${id}`);

  check(res, {
    'status 200': (r) => r.status === 200,
  });

  sleep(1);
}