import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '30s', target: 100 }, // Ramp up to 100 VUs
    { duration: '1m', target: 100 },  // Hold at 100 VUs
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<500'], // 95% < 100ms, 99% < 500ms
    errors: ['rate<0.1'], // Error rate < 10%
    http_req_failed: ['rate<0.05'], // Request failure rate < 5%
  },
};

// Test configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

// Sample batch and client IDs (these should exist in your test data)
const TEST_BATCHES = [
  { batchId: '11111111-1111-1111-1111-111111111111', clientId: '217be7c8-679c-4e08-bffc-db3451bdcdbf' },
  { batchId: '22222222-2222-2222-2222-222222222222', clientId: '317be7c8-679c-4e08-bffc-db3451bdcdbf' },
  { batchId: '33333333-3333-3333-3333-333333333333', clientId: '417be7c8-679c-4e08-bffc-db3451bdcdbf' },
];

function generateCustomerId() {
  // Generate random UUIDs for customers to test rule checking
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
  // 80% code redemptions, 20% batch queries
  if (Math.random() < 0.8) {
    testCodeRedemption();
  } else {
    testBatchQuery();
  }

  sleep(0.1); // Small delay between requests
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
    tags: { name: 'code_redemption' },
  };

  const response = http.post(`${BASE_URL}/api/v1/code/redeem`, payload, params);

  const success = check(response, {
    'redemption status is 200, 403, or 404': (r) => [200, 403, 404].includes(r.status),
    'redemption response time < 200ms': (r) => r.timings.duration < 200,
    'redemption has valid JSON response': (r) => {
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
    console.log(`Redemption failed: ${response.status} ${response.body}`);
  } else {
    errorRate.add(0);
  }

  // Additional checks for successful redemptions
  if (response.status === 200) {
    check(response, {
      'redemption returns code': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.code && body.code.length > 0;
        } catch (e) {
          return false;
        }
      },
    });
  }
}

function testBatchQuery() {
  const params = {
    tags: { name: 'batch_query' },
  };

  const response = http.get(`${BASE_URL}/api/v1/batches`, params);

  const success = check(response, {
    'batch query status is 200': (r) => r.status === 200,
    'batch query response time < 100ms': (r) => r.timings.duration < 100,
    'batch query returns array': (r) => {
      try {
        const body = JSON.parse(r.body);
        return Array.isArray(body);
      } catch (e) {
        return false;
      }
    },
  });

  if (!success) {
    errorRate.add(1);
    console.log(`Batch query failed: ${response.status} ${response.body}`);
  } else {
    errorRate.add(0);
  }
}

export function handleSummary(data) {
  return {
    'baseline-results.json': JSON.stringify(data, null, 2),
    stdout: `
    ========== BASELINE TEST RESULTS ==========

    Requests: ${data.metrics.http_reqs.values.count}
    Duration: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms avg

    Performance:
    - p50: ${data.metrics.http_req_duration.values['p(50)'].toFixed(2)}ms
    - p95: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms
    - p99: ${data.metrics.http_req_duration.values['p(99)'].toFixed(2)}ms

    Error Rate: ${(data.metrics.errors.values.rate * 100).toFixed(2)}%

    ==========================================
    `,
  };
}