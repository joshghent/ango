import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL = 'http://localhost:9001';
const errorRate = new Rate('errors');

export let options = {
  stages: [
    { duration: '10s', target: 10 },  // Normal load
    { duration: '30s', target: 200 }, // Spike to 200 users
    { duration: '10s', target: 10 },  // Scale down to normal
    { duration: '30s', target: 10 },  // Stay at normal
    { duration: '10s', target: 0 },   // Scale down to 0
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'], // 95% of requests under 2s
    errors: ['rate<0.3'],              // Error rate under 30%
  },
};

function generateCustomerId() {
  return 'spike-' + Math.random().toString(36).substr(2, 9);
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

  sleep(0.1); // Very short sleep for spike test
}