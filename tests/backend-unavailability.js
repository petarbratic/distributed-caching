import http from 'k6/http';
import { check } from 'k6';

const gatewayURL = 'http://localhost:8080';
const backendFaultURL = 'http://localhost:8081/fault';

const faultStartTime = '120s';
const faultDurationMs = 13000;

export const options = {
  scenarios: {
    moderateSkewLoad: {
      executor: 'ramping-arrival-rate',
      exec: 'generateLoad',

      startRate: 5,
      timeUnit: '1s',

      preAllocatedVUs: 100,
      maxVUs: 250,

      stages: [
        {
          target: 77,
          duration: '15s',
        },
        {
          target: 77,
          duration: '225s',
        },
      ],
    },

    backendFault: {
      executor: 'per-vu-iterations',
      exec: 'activateBackendFault',

      vus: 1,
      iterations: 1,

      startTime: faultStartTime,
      maxDuration: '5s',
    },
  },
};
const firstKey = 1;
const keyCount = 100;
const skew = 0.8;
//const sleepConstant = 0.5;


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

export function generateLoad() {
  const id = selectZipfKey();

  const response = http.get(`${gatewayURL}/api/backend/${id}`);

  check(response, {
    'status is 200': (result) =>
      result.status === 200,
  });
}

export function activateBackendFault() {
  const response = http.post(
    backendFaultURL,
    JSON.stringify({
      durationMs: faultDurationMs,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
      },

      tags: {
        request_type: 'fault-control',
      },
    },
  );

  check(response, {
    'backend fault activated': (result) =>
      result.status === 204,
  });
}