cmake_minimum_required(VERSION 3.20)

project(var_studio_demos LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 REQUIRED COMPONENTS Widgets)

add_subdirectory(medical-vitals)
add_subdirectory(automotive-cluster)
