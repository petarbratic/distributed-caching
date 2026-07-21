import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 5,
  duration: '90s',
};

export default function () {
  const id = Math.floor(Math.random() * 5) + 2000;

  const res = http.get(`http://localhost:8080/api/backend/${id}`);

  check(res, {
    'status 200': (r) => r.status === 200,
  });

  sleep(1);
}