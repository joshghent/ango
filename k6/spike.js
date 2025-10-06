import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '10s', target: 100 },   // Start with normal load
    { duration: '1s', target: 5000 },   // Sudden spike to 5000 VUs
    { duration: '30s', target: 5000 },  // Hold spike for 30 seconds
    { duration: '10s', target: 100 },   // Return to normal
    { duration: '30s', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'], // Very lenient during spike
    errors: ['rate<0.5'], // Allow higher error rate during spike
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

const TEST_BATCHES = [
  { batchId: '11111111-1111-1111-1111-111111111111', clientId: '217be7c8-679c-4e08-bffc-db3451bdcdbf' },
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
  const batch = TEST_BATCHES[0]; // Use single batch to create contention
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
    tags: { name: 'spike_test' },
  };

  const response = http.post(`${BASE_URL}/api/v1/code/redeem`, payload, params);

  const success = check(response, {
    'spike test status is valid': (r) => [200, 403, 404, 503, 408].includes(r.status), // Include timeout
    'spike test completes within 3s': (r) => r.timings.duration < 3000,
  });

  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }

  // No sleep - maximum pressure
}

export function handleSummary(data) {
  return {
    'spike-results.json': JSON.stringify(data, null, 2),
    stdout: `
    ========== SPIKE TEST RESULTS ==========

    Peak RPS: ${data.metrics.http_reqs.values.rate.toFixed(2)}
    Total Requests: ${data.metrics.http_reqs.values.count}

    Spike Performance:
    - p50: ${data.metrics.http_req_duration.values['p(50)'].toFixed(2)}ms
    - p95: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms
    - p99: ${data.metrics.http_req_duration.values['p(99)'].toFixed(2)}ms

    Error Rate: ${(data.metrics.errors.values.rate * 100).toFixed(2)}%
    System survived spike: ${data.metrics.errors.values.rate < 0.8 ? 'YES' : 'NO'}

    =========================================
    `,
  };
}