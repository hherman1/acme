#!/bin/bash

# install gosh executables
cd gosh
go install ./cmd/liner

# installs all the executables in src/
cd ../src/doc
go install .
