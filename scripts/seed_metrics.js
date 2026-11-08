const http = require('http');
const fs = require('fs');
const path = require('path');

async function get(url, headers = {}) {
  return new Promise((resolve, reject) => {
    http.get(url, { headers }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => resolve({ status: res.statusCode, data }));
    }).on('error', reject);
  });
}

async function post(url, body) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify(body);
    const req = http.request(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(data),
      }
    }, (res) => {
      let resp = '';
      res.on('data', chunk => resp += chunk);
      res.on('end', () => resolve({ status: res.statusCode, data: resp }));
    });
    req.on('error', reject);
    req.write(data);
    req.end();
  });
}

async function main() {
  const blobsDir = path.resolve('B:/workgit/regcraft/testcache/blobs/sha256');
  const subdirs = fs.readdirSync(blobsDir).filter(f => fs.statSync(path.join(blobsDir, f)).isDirectory());

  const blobDigests = [];
  for (const sub of subdirs) {
    const files = fs.readdirSync(path.join(blobsDir, sub));
    for (const f of files) {
      if (f.length === 64) {
        blobDigests.push(f);
      }
    }
  }

  console.log(`Found ${blobDigests.length} valid 64-char blobs in cache. Simulating layer hits...`);

  // Request blobs to simulate 10 Gbps cache hits
  for (const digest of blobDigests.slice(0, 20)) {
    for (let i = 0; i < 3; i++) {
      await get(`http://localhost:5000/v2/library/alpine/blobs/sha256:${digest}`);
    }
  }

  // Pin postgres:16-alpine
  await post('http://localhost:5000/api/v1/catalog/library/postgres/tags/16-alpine/pin', { pinned: true });

  const metrics = await get('http://localhost:5000/api/v1/metrics');
  console.log('Metrics:', metrics.data);
}

main().catch(console.error);
