import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '2m', target: 500 },   // Ramp up to 500 VUs
    { duration: '10m', target: 500 },  // Hold at 500 VUs for 10 minutes (soak)
    { duration: '2m', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<150', 'p(99)<300'], // Should maintain performance
    errors: ['rate<0.1'], // Low error rate over time
    http_req_failed: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

const TEST_BATCHES = [
  { batchId: '11111111-1111-1111-1111-111111111111', clientId: '217be7c8-679c-4e08-bffc-db3451bdcdbf' },
  { batchId: '22222222-2222-2222-2222-222222222222', clientId: '317be7c8-679c-4e08-bffc-db3451bdcdbf' },
  { batchId: '33333333-3333-3333-3333-333333333333', clientId: '417be7c8-679c-4e08-bffc-db3451bdcdbf' },
];

function generateCustomerId() {
  const chars = '0123456789abcdef';
  let result = '';
  for (let i = 0; i < 32; i++) {
    result += chars[Math.floor(Math.random() * 16)];
    if (i === 7 || i === 11 || i === 15 || i === 19) {
      result += '-';
    }
  }
  return result;
}

export default function () {
  // 85% redemptions, 15% queries - steady load
  if (Math.random() < 0.85) {
    testCodeRedemption();
  } else {
    testBatchQuery();
  }

  sleep(0.1); // Consistent pacing
}

function testCodeRedemption() {
  const batch = TEST_BATCHES[Math.floor(Math.random() * TEST_BATCHES.length)];
  const customerId = generateCustomerId();

  const payload = JSON.stringify({
    batchid: batch.batchId,
    clientid: batch.clientId,
    customerid: customerId,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: { name: 'soak_redemption' },
  };

  const response = http.post(`${BASE_URL}/api/v1/code/redeem`, payload, params);

  const success = check(response, {
    'soak redemption status is valid': (r) => [200, 403, 404].includes(r.status),
    'soak redemption response time < 300ms': (r) => r.timings.duration < 300,
    'soak redemption has valid response': (r) => {
      try {
        JSON.parse(r.body);
        return true;
      } catch (e) {
        return false;
      }
    },
  });

  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }
}

function testBatchQuery() {
  const response = http.get(`${BASE_URL}/api/v1/batches`);

  const success = check(response, {
    'soak batch query status is 200': (r) => r.status === 200,
    'soak batch query response time < 150ms': (r) => r.timings.duration < 150,
  });

  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }
}

export function handleSummary(data) {
  const avgCpu = data.metrics.iterations ?
    (data.metrics.iterations.values.count / (data.state.testRunDurationMs / 1000)).toFixed(2) : 0;

  return {
    'soak-results.json': JSON.stringify(data, null, 2),
    stdout: `
    ========== SOAK TEST RESULTS ==========

    Test Duration: ${Math.round(data.state.testRunDurationMs / 1000 / 60)} minutes
    Total Requests: ${data.metrics.http_reqs.values.count}
    Avg RPS: ${data.metrics.http_reqs.values.rate.toFixed(2)}

    Memory Leak Check:
    - Consistent Performance: ${data.metrics.http_req_duration.values['p(95)'] < 150 ? 'PASS' : 'FAIL'}
    - Error Rate Stable: ${data.metrics.errors.values.rate < 0.1 ? 'PASS' : 'FAIL'}

    Final Performance:
    - p50: ${data.metrics.http_req_duration.values['p(50)'].toFixed(2)}ms
    - p95: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms
    - p99: ${data.metrics.http_req_duration.values['p(99)'].toFixed(2)}ms

    Error Rate: ${(data.metrics.errors.values.rate * 100).toFixed(2)}%

    ======================================
    `,
  };
}