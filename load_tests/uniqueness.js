import http from 'k6/http';
import { check, fail } from 'k6';

const BASE_URL = 'http://localhost:9001';

export let options = {
  duration: '10s',
  vus: 5,
  thresholds: {
    http_req_duration: ['p(95)<500'],
    checks: ['rate>0.99'], // 99% of checks must pass
  },
};

const codes = new Set(); // Track unique codes

function generateCustomerId() {
  // Generate a proper UUID format
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
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
    'has code': (r) => r.json('code') !== undefined,
  });

  if (success && response.json('code')) {
    const code = response.json('code');

    // Check for uniqueness
    if (codes.has(code)) {
      console.error(`DUPLICATE CODE DETECTED: ${code}`);
      fail('Duplicate code returned');
    } else {
      codes.add(code);
      console.log(`Unique code received: ${code} (total: ${codes.size})`);
    }
  }
}