qt_add_executable(medical-vitals
  main.cpp
)

target_link_libraries(medical-vitals PRIVATE Qt6::Widgets)

install(TARGETS medical-vitals
  RUNTIME DESTINATION /opt/var-studio-demos/bin
)

install(FILES manifest.json
  DESTINATION /opt/var-studio-demos/manifests
)
