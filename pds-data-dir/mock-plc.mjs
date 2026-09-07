import http from 'node:http'
const ops = new Map()
function doc(did, op) {
  const vm = op.verificationMethods || {}
  return {
    '@context': ['https://www.w3.org/ns/did/v1','https://w3id.org/security/multikey/v1'],
    id: did, alsoKnownAs: op.alsoKnownAs || [],
    verificationMethod: Object.entries(vm).map(([k,v]) => ({ id: `${did}#${k}`, type: 'Multikey', controller: did, publicKeyMultibase: v.replace(/^did:key:/, '') })),
    service: Object.entries(op.services || {}).map(([k,v]) => ({ id: `#${k}`, type: v.type, serviceEndpoint: v.endpoint })),
  }
}
http.createServer((req, res) => {
  let body = ''
  req.on('data', d => body += d)
  req.on('end', () => {
    const [, didEnc, ...rest] = req.url.split('/')
    const did = decodeURIComponent(didEnc || '')
    console.log(req.method, req.url)
    if (req.url === '/_health') { res.writeHead(200, {'content-type':'application/json'}); return res.end('{"version":"mock"}') }
    if (req.method === 'POST') { ops.set(did, JSON.parse(body)); res.writeHead(200); return res.end() }
    const op = ops.get(did)
    if (!op) { res.writeHead(404, {'content-type':'application/json'}); return res.end('{"message":"DID not registered: '+did+'"}') }
    res.writeHead(200, {'content-type':'application/json'})
    const sub = rest.join('/')
    if (sub === '') return res.end(JSON.stringify(doc(did, op)))
    if (sub === 'data') return res.end(JSON.stringify({ did, rotationKeys: op.rotationKeys, verificationMethods: op.verificationMethods, alsoKnownAs: op.alsoKnownAs, services: op.services }))
    if (sub === 'log/last') return res.end(JSON.stringify(op))
    if (sub === 'log') return res.end(JSON.stringify([op]))
    res.end(JSON.stringify(op))
  })
}).listen(2582, () => console.log('mock plc on 2582'))
