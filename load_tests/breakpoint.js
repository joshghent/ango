import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL = 'http://localhost:9001';
const errorRate = new Rate('errors');

export let options = {
  executor: 'ramping-arrival-rate',
  stages: [
    { duration: '2m', target: 100 },   // 100 req/s
    { duration: '2m', target: 200 },   // 200 req/s
    { duration: '2m', target: 400 },   // 400 req/s
    { duration: '2m', target: 800 },   // 800 req/s
    { duration: '2m', target: 1600 },  // 1600 req/s
    { duration: '2m', target: 3200 },  // 3200 req/s
    { duration: '10m', target: 0 },    // Scale down
  ],
  preAllocatedVUs: 50,
  maxVUs: 500,
  thresholds: {
    http_req_duration: ['p(95)<2000'], // 95% of requests under 2s
    errors: ['rate<0.5'],              // Error rate under 50%
  },
};

function generateCustomerId() {
  return 'breakpoint-' + Math.random().toString(36).substr(2, 9);
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
    'response time < 2000ms': (r) => r.timings.duration < 2000,
  });

  errorRate.add(!success);
}