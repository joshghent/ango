import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  executor: 'ramping-arrival-rate',
  stages: [
    { duration: '5m', target: 100 },   // Start with 100 RPS
    { duration: '5m', target: 500 },   // Ramp to 500 RPS
    { duration: '5m', target: 1000 },  // Ramp to 1000 RPS
    { duration: '5m', target: 2000 },  // Ramp to 2000 RPS
    { duration: '5m', target: 3000 },  // Ramp to 3000 RPS
    { duration: '5m', target: 5000 },  // Push to 5000 RPS
    { duration: '5m', target: 7000 },  // Continue pushing
    { duration: '5m', target: 10000 }, // Find the breaking point
  ],
  preAllocatedVUs: 500,  // Pre-allocate VUs
  maxVUs: 2000,          // Max VUs allowed
  thresholds: {
    // More lenient thresholds to find breaking point
    errors: ['rate<0.9'], // Allow up to 90% errors to find limit
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
  const batch = TEST_BATCHES[0];
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
    tags: { name: 'breakpoint_test' },
    timeout: '10s', // Longer timeout for high load
  };

  const response = http.post(`${BASE_URL}/api/v1/code/redeem`, payload, params);

  const success = check(response, {
    'breakpoint status is valid': (r) => r.status !== 0 && r.status < 600, // Any HTTP response
    'breakpoint completes within 10s': (r) => r.timings.duration < 10000,
  });

  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }

  // Track when system starts degrading
  if (response.timings.duration > 5000) {
    console.log(`High latency detected: ${response.timings.duration}ms at ${__ITER} iterations`);
  }

  if (response.status >= 500) {
    console.log(`Server error: ${response.status} at ${__ITER} iterations`);
  }
}

export function handleSummary(data) {
  const maxRps = Math.max(...Object.values(data.metrics.http_reqs.values || {}));
  const finalErrorRate = data.metrics.errors.values.rate;

  let breakingPoint = 'Not reached';
  if (finalErrorRate > 0.5) {
    breakingPoint = `~${data.metrics.http_reqs.values.rate.toFixed(0)} RPS`;
  }

  return {
    'breakpoint-results.json': JSON.stringify(data, null, 2),
    stdout: `
    ========== BREAKPOINT TEST RESULTS ==========

    Maximum Sustainable RPS: ${data.metrics.http_reqs.values.rate.toFixed(2)}
    Breaking Point: ${breakingPoint}
    Total Requests: ${data.metrics.http_reqs.values.count}

    Performance at Peak:
    - p50: ${data.metrics.http_req_duration.values['p(50)'].toFixed(2)}ms
    - p95: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms
    - p99: ${data.metrics.http_req_duration.values['p(99)'].toFixed(2)}ms

    Final Error Rate: ${(finalErrorRate * 100).toFixed(2)}%

    System Capacity Found: ${finalErrorRate < 0.1 ? 'System can handle more' : 'Breaking point identified'}

    ============================================
    `,
  };
}