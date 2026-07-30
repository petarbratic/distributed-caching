import http from 'k6/http';
import { check, fail, sleep } from 'k6';

const baseURL = 'http://localhost:8080';

const firstKey = 1;
const keyCount = 6;
const skew = 1.2;

const l2TTLSeconds = 15;
const expirationMarginSeconds = 0.25;
const sleepBetweenRequests = 0.5;

export const options = {
  vus: 100,
  duration: '30s',
};

const cumulativeProbabilities = createZipfDistribution(
  keyCount,
  skew,
);

export function setup() {
  const requests = [];

  for (let key = firstKey; key < firstKey + keyCount; key++) {
    requests.push(`${baseURL}/api/backend/${key}`);
  }

  const responses = http.batch(requests);

  for (const response of responses) {
    const successful = check(response, {
      'cache warm-up status is 200': (res) => res.status === 200,
    });

    if (!successful) {
      fail(
        `Cache warm-up failed with status ${response.status}`,
      );
    }
  }

  console.log(
    `Keys ${firstKey}-${firstKey + keyCount - 1} warmed. ` +
    `Waiting ${l2TTLSeconds + expirationMarginSeconds}s for synchronized expiry.`,
  );

  sleep(l2TTLSeconds + expirationMarginSeconds);
}

export default function () {
  const key = selectZipfKey();

  const response = http.get(`${baseURL}/api/backend/${key}`);

  check(response, {'status is 200': (res) => res.status === 200});

  sleep(sleepBetweenRequests);
}

function createZipfDistribution(numberOfKeys, zipfSkew) {
  const weights = [];

  for (let rank = 1; rank <= numberOfKeys; rank++) {
    weights.push(1 / Math.pow(rank, zipfSkew));
  }

  const totalWeight = weights.reduce(
    (sum, weight) => sum + weight,
    0,
  );

  const cumulativeProbabilities = [];
  let cumulativeProbability = 0;

  for (const weight of weights) {
    cumulativeProbability += weight / totalWeight;
    cumulativeProbabilities.push(cumulativeProbability);
  }

  return cumulativeProbabilities;
}

function selectZipfKey() {
  const randomValue = Math.random();

  for (
    let index = 0;
    index < cumulativeProbabilities.length;
    index++
  ) {
    if (randomValue <= cumulativeProbabilities[index]) {
      return firstKey + index;
    }
  }

  return firstKey + keyCount - 1;
}