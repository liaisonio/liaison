const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const ts = require('typescript');
const source = fs.readFileSync(require.resolve('../src/pages/WebData/statementUtils.ts'), 'utf8');
const moduleContext = {exports:{},require:()=>({formatCellValue:String})};
vm.runInNewContext(ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText,moduleContext);
const build = moduleContext.exports.buildExplainStatement;
for(const input of ['SELECT * FROM ORDERS', 'SELECT * FROM ORDERS;', 'EXPLAIN SELECT * FROM ORDERS', 'EXPLAIN PLAN FOR SELECT * FROM ORDERS;']) {
 assert.equal(build(input,'oracle'),'EXPLAIN PLAN FOR SELECT * FROM ORDERS;');
}
assert.equal(build('SELECT 1','mysql'),'EXPLAIN SELECT 1;');
assert.equal(build('EXPLAIN SELECT 1;','postgresql'),'EXPLAIN SELECT 1;');
console.log('Oracle Explain frontend regression passed');
