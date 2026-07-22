import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 50,
  duration: '120s',
};
const firstKey = 1;
const keyCount = 2500;
const skew = 2.2;
const sleepConstant = 0.5;


const weights = [];

for (let rank = 1; rank <= keyCount; rank++) {
  weights.push(1 / Math.pow(rank, skew));
}

const totalWeight = weights.reduce((sum, weight) => sum + weight, 0);

const cumulativeProbabilities = [];

let cumulativeSum = 0;

for (const weight of weights) {
  cumulativeSum += weight / totalWeight;
  cumulativeProbabilities.push(cumulativeSum);
}

function selectZipfKey() {
  const randomValue = Math.random();

  for (let index = 0; index < cumulativeProbabilities.length; index++) {
    if (randomValue <= cumulativeProbabilities[index]) {
      return firstKey + index;
    }
  }

  return firstKey + keyCount - 1;
}

export default function () {
  const id = selectZipfKey();

  const res = http.get(`http://localhost:8080/api/backend/${id}`);

  check(res, {
    'status 200': (r) => r.status === 200,
  });

  sleep(sleepConstant);
}