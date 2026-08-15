import http from 'k6/http';
import { check } from 'k6';

const baseURL = 'http://localhost:8080';

const firstKey = 1;
const keyCount = 100;
const skew = 0.8;

const baselineRequestRate = 20;
const burstRequestRate = 180;

const baselineDuration = '30s';
const rampUpDuration = '1s';
const burstDuration = '45s';
const rampDownDuration = '1s';
const recoveryDuration = '45s';

const cumulativeProbabilities = createZipfDistribution(
  keyCount,
  skew,
);

export const options = {
  scenarios: {
    burstLoad: {
      executor: 'ramping-arrival-rate',
      startRate: baselineRequestRate,
      timeUnit: '1s',

      preAllocatedVUs: 200,
      maxVUs: 600,

      stages: [
        {
          target: baselineRequestRate,
          duration: baselineDuration,
        },
        {
          target: burstRequestRate,
          duration: rampUpDuration,
        },
        {
          target: burstRequestRate,
          duration: burstDuration,
        },
        {
          target: baselineRequestRate,
          duration: rampDownDuration,
        },
        {
          target: baselineRequestRate,
          duration: recoveryDuration,
        },
      ],
    },
  },
};

export default function () {
  const key = selectZipfKey();

  const response = http.get(
    `${baseURL}/api/backend/${key}`,
  );

  check(response, {
    'status is 200': (res) => res.status === 200,
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