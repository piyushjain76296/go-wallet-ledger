import http from 'k6/http';
import { check, sleep } from 'k6';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

export const options = {
    scenarios: {
        high_throughput_transfers: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '30s', target: 50 },  // Ramp up to 50 concurrent virtual users
                { duration: '1m', target: 50 },   // Sustain
                { duration: '30s', target: 100 }, // Spike to 100
                { duration: '1m', target: 100 },  // Sustain
                { duration: '30s', target: 0 },   // Ramp down
            ],
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<200'], // 95% of requests should complete within 200ms
        http_req_failed: ['rate<0.01'],   // Error rate should be less than 1%
    },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080/api/v1';
const JWT_TOKEN = __ENV.JWT_TOKEN || ''; // Pass via environment for load test
const WALLET_1 = __ENV.WALLET_1 || '';
const WALLET_2 = __ENV.WALLET_2 || '';

export default function () {
    const payload = JSON.stringify({
        source_wallet_id: Math.random() > 0.5 ? WALLET_1 : WALLET_2,
        destination_wallet_id: Math.random() > 0.5 ? WALLET_2 : WALLET_1,
        amount: Math.floor(Math.random() * 10) + 1,
        currency: 'USD',
    });

    const headers = {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${JWT_TOKEN}`,
        'Idempotency-Key': `k6-test-${randomString(16)}`,
    };

    const res = http.post(`${BASE_URL}/transfers`, payload, { headers });

    // Since we randomly swap source/dest, if they happen to be the same, 
    // the API returns 400. We don't count that as a failure for the infrastructure.
    check(res, {
        'status is 201 or 400 (same wallet)': (r) => r.status === 201 || r.status === 400,
        'latency < 200ms': (r) => r.timings.duration < 200,
    });

    // Small sleep to prevent overwhelming the local machine running the test
    sleep(0.1); 
}
