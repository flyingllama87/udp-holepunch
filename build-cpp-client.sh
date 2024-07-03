g++ client.cpp -o bin/client-gcc -std=c++11 -pthread
clang++ client.cpp -o bin/client-clang -std=c++11 -stdlib=libc++ -pthread
