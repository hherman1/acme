#!/bin/bash

# install gosh executables
cd gosh
go install ./cmd/liner
go install ./cmd/inpage
go install ./cmd/pidlock


# installs all the executables in src/
cd ../src/doc
go install .
