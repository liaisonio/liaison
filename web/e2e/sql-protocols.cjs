// Pure frontend regression checks; no browser or database required.
// Run: node web/e2e/sql-protocols.cjs
const assert = require('node:assert/strict');
const fs = require('node:fs');
const ts = require('typescript');

require.extensions['.ts'] = (module, filename) => {
  const source = fs.readFileSync(filename, 'utf8');
  module._compile(ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText, filename);
};

const access = require('../src/constants/accessTypes.ts');
const { isSQLProtocol, protocolLabels } = require('../src/pages/WebData/protocol.ts');
const { tlsOptionsForProtocol } = require('../src/pages/WebData/connection.ts');
const { sqlQualifiedName, sqlQuoteIdent } = require('../src/pages/WebData/objectCommands.ts');

for (const protocol of ['mysql', 'mariadb', 'postgresql', 'sqlserver']) {
  assert.equal(isSQLProtocol(protocol), true);
  assert.ok(protocolLabels[protocol]);
  const webType = 'web' + protocol;
  assert.equal(access.applicationTypeForAccess(webType), protocol);
  assert.equal(access.accessProtocolForType(webType), 'web');
  assert.equal(access.getProxyAccessType({ access_protocol: 'web', application: { application_type: protocol } }), webType);
  assert.deepEqual(access.accessTypesForApplication(protocol).map(item => item.value), ['tcp', webType]);
  assert.ok(tlsOptionsForProtocol(protocol, (_, en) => en).some(item => item.value === 'require'));
}
assert.equal(sqlQuoteIdent('mariadb', 'odd`name'), '`odd``name`');
assert.equal(sqlQualifiedName('mariadb', { database: 'app', name: 'orders' }), '`app`.`orders`');
assert.equal(sqlQualifiedName('postgresql', { schema: 'sales', name: 'orders' }), '"sales"."orders"');
assert.equal(isSQLProtocol('mongodb'), false);
assert.equal(sqlQualifiedName('sqlserver', { name: 'orders' }), '[dbo].[orders]');
assert.equal(sqlQuoteIdent('sqlserver', 'odd]name'), '[odd]]name]');
const commands = require('../src/pages/WebData/objectCommands.ts');
const filter = commands.buildSQLFilterCommand('sqlserver', {name:'orders', columns:[{column_name:'id', data_type:'int'}]}, {limit:20, conditions:[{field:'id',operator:'eq',value:'1'}]});
assert.match(filter, /SELECT TOP \(20\)/);
assert.ok(!filter.includes('LIMIT'));
console.log('SQL protocol frontend checks passed (MySQL, MariaDB, PostgreSQL, SQL Server).');
