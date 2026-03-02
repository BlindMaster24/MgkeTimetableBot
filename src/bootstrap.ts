const bufferModule = require('buffer');

if (!bufferModule.SlowBuffer) {
    bufferModule.SlowBuffer = bufferModule.Buffer;
}

(global as any).SlowBuffer = bufferModule.SlowBuffer;

import './index';
