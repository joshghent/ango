import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 1,
  iterations: 1,
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

// Test batches to create
const TEST_BATCHES = [
  {
    name: 'Load Test Batch 1',
    rules: JSON.stringify({ maxpercustomer: 5, timelimit: 30 }),
    codes: 50000, // 50k codes for high-volume testing
    batchId: '11111111-1111-1111-1111-111111111111',
    clientId: '217be7c8-679c-4e08-bffc-db3451bdcdbf',
  },
  {
    name: 'Load Test Batch 2',
    rules: JSON.stringify({ maxpercustomer: 3, timelimit: 7 }),
    codes: 25000,
    batchId: '22222222-2222-2222-2222-222222222222',
    clientId: '317be7c8-679c-4e08-bffc-db3451bdcdbf',
  },
  {
    name: 'Load Test Batch 3',
    rules: JSON.stringify({ maxpercustomer: 1, timelimit: 1 }),
    codes: 10000,
    batchId: '33333333-3333-3333-3333-333333333333',
    clientId: '417be7c8-679c-4e08-bffc-db3451bdcdbf',
  },
];

function generateCode() {
  return Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15);
}

function createCSVContent(batch) {
  let csv = 'client_id,code\n';
  for (let i = 0; i < batch.codes; i++) {
    csv += `${batch.clientId},${generateCode()}\n`;
  }
  return csv;
}

export default function () {
  console.log('Starting test data seeding...');

  for (const batch of TEST_BATCHES) {
    console.log(`Creating batch: ${batch.name} with ${batch.codes} codes`);

    // Create CSV content
    const csvContent = createCSVContent(batch);
    const csvBlob = new Blob([csvContent], { type: 'text/csv' });

    // Prepare form data
    const formData = {
      batch_name: batch.name,
      rules: batch.rules,
      file: http.file(csvBlob, `${batch.name.replace(/\s+/g, '_')}.csv`, 'text/csv'),
    };

    // Upload codes
    const response = http.post(`${BASE_URL}/api/v1/codes/upload`, formData);

    const success = check(response, {
      [`${batch.name} upload successful`]: (r) => r.status === 200,
      [`${batch.name} upload response valid`]: (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.message && body.message.includes('successfully');
        } catch (e) {
          return false;
        }
      },
    });

    if (success) {
      console.log(`✓ Successfully created batch: ${batch.name}`);
    } else {
      console.log(`✗ Failed to create batch: ${batch.name} - Status: ${response.status}`);
      console.log(`Response: ${response.body}`);
    }
  }

  console.log('Test data seeding completed!');
}

export function handleSummary(data) {
  return {
    stdout: `
    ========== TEST DATA SEEDING RESULTS ==========

    Batches Created: ${TEST_BATCHES.length}
    Total Codes: ${TEST_BATCHES.reduce((sum, batch) => sum + batch.codes, 0)}

    Seeding successful: ${data.metrics.checks ?
      (data.metrics.checks.values.passes / data.metrics.checks.values.count * 100).toFixed(1) + '%' :
      'N/A'}

    ===============================================
    `,
  };
}