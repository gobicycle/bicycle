# TON Func Contract Compiler

This package aims to be headless FunC compiler. It compiles you FunC source code straight to BOC.

The resulting BOC is encoded to base64 format. Also result has fift code of your contract.

## How it works

Internally this package uses both FunC compiler and Fift interpreter combined to one lib and compiled to WASM.

## Usage example

```typescript
import {compileFunc} from 'ton-compiler';
import {Cell} from 'ton';


async function buildCodeCell() {
    let conf = {
        optLevel: 2,
        sources: {
            "stdlib.fc": "<stdlibCode>",
            "yourContract": "<contractCode>"
            // another files if needed
        }
    };

    let result = await funcCompile(conf);

    if (result.status === 'error') {
        // ...
        return;
    }

    let codeCell = Cell.fromBoc(Buffer.from(result.codeBoc, "base64"))[0];

    return codeCell;
}
```

Also you can get compiler version if you want notify us about some issues.


```typescript
import {compilerVersion} from 'ton-compiler';

async function getCompilerVersion() {

    let version = await compilerVersion();

    return version;
}
```

## WEB integration

The WASM library contains tools wich allow to work in web and nodejs both, so you can use it in browsers too. So you sould configure your webpack for resolve some nodejs requires.

```
// webpack.config.js
resolve: {
    fallback: {
        fs: false,
        path: false,
        crypto: false
    }
}
```

Also you should configure webpack for including *.wasm file at build.