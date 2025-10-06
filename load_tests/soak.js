import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL = 'http://localhost:9001';
const errorRate = new Rate('errors');

export let options = {
  stages: [
    { duration: '2m', target: 30 },   // Ramp up to 30 users
    { duration: '10m', target: 30 },  // Stay at 30 users for 10 minutes
    { duration: '2m', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests under 500ms
    errors: ['rate<0.05'],            // Error rate under 5%
  },
};

function generateCustomerId() {
  return 'soak-' + Math.random().toString(36).substr(2, 9);
}

export default function() {
  const customerId = generateCustomerId();
  const payload = JSON.stringify({
    batchid: '11111111-1111-1111-1111-111111111111',
    clientid: '217be7c8-679c-4e08-bffc-db3451bdcdbf',
    customerid: customerId
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const response = http.post(`${BASE_URL}/api/v1/code/redeem`, payload, params);

  const success = check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });

  errorRate.add(!success);

  sleep(1); // Normal pace for soak test
}