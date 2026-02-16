provider "sqlserver" {
    debug = true
    host = "gltftest-managedsqlinstance.303019bcb85a.database.windows.net"
    azuread_default_chain_auth {}
}

resource "sqlserver_login" "test_login" {
  external_login {  
    login_name = "test-identity"
    external_login_type = "user"
  }
  timeouts {
      read = "10m"
      create = "10m"
      update = "10m"
      delete = "10m"
    }
}

resource "sqlserver_user" "test_user" {
  database = "test"
  login_name = "test-identity"
  username = "test-identity"
  roles = ["db_datareader", "db_datawriter"]
  
  depends_on = [ sqlserver_login.test_login ]
}
