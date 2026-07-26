'use strict';

const fs = require('fs');
const path = require('path');
const generateEvb = require('../build/evb-tools/node_modules/generate-evb');

const root = path.resolve(__dirname, '..');
const inputExe = path.join(root, 'build', 'RestaurantFinance.exe');
const customerDir = path.join(root, 'build', 'customer');
const contentDir = path.join(root, 'build', 'enigma-content');
const projectFile = path.join(customerDir, 'RestaurantFinance-Temporary.evb');
const outputExe = path.join(customerDir, 'RestaurantFinance-Temporary.exe');

if (!fs.existsSync(inputExe)) {
  throw new Error(`Input executable not found: ${inputExe}`);
}

fs.mkdirSync(customerDir, { recursive: true });
fs.mkdirSync(contentDir, { recursive: true });

if (fs.existsSync(outputExe)) {
  fs.rmSync(outputExe);
}

generateEvb(projectFile, inputExe, outputExe, contentDir, {
  evbOptions: {
    deleteExtractedOnExit: true,
    compressFiles: true,
    shareVirtualSystem: false,
    mapExecutableWithTemporaryFile: true,
    allowRunningOfVirtualExeFiles: true,
  },
});

console.log(projectFile);
