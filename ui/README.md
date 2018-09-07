# UI

## Setup

Get dependencies

* nvm
* parcel
* tsc - the typescript compiler

## Hacking

To hack on the UI, you need two terminals. In the first terminal, run

```
parcel watch index.html
```

This will compile a **dist** directory. In the other terminal, from the 
repository root, run

```
./co-chair serve --bypassAuth0 --webAssetsPath ui/dist
```

Then visit https://localhost:2016 in your browser. You will need to 
refresh manually, for now.

