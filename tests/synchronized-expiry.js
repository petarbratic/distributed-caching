import http from 'k6/http';
import { check, fail } from 'k6';

const baseURL = 'http://localhost:8080';

const firstKey = 1;
const keyCount = 5;
const skew = 1.2;

export const options = {
  scenarios: {
    synchronizedExpiryLoad: {
      executor: 'constant-arrival-rate',

      rate: 80,
      timeUnit: '1s',
      duration: '60s',

      preAllocatedVUs: 100,
      maxVUs: 250,
    },
  },
};

const cumulativeProbabilities = createZipfDistribution(
  keyCount,
  skew,
);

export function setup() {
  const requests = [];

  for (let key = firstKey; key < firstKey + keyCount; key++) {
    requests.push({
      method: 'GET',
      url: `${baseURL}/api/backend/${key}`,
    });
  }

  const responses = http.batch(requests);

  for (const response of responses) {
    const successful = check(response, {
      'cache warm-up status is 200': (result) =>
        result.status === 200,
    });

    if (!successful) {
      fail(
        `Cache warm-up failed with status ${response.status}`,
      );
    }
  }

  console.log(
    `Keys ${firstKey}-${firstKey + keyCount - 1} ` +
    'were loaded into the cache with synchronized expiration.',
  );
}

export default function () {
  const key = selectZipfKey();

  const response = http.get(`${baseURL}/api/backend/${key}`);

  check(response, {
    'status is 200': (result) => result.status === 200,
  });
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