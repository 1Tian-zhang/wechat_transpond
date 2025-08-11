<template>
  <div class="dashboard">
    <div class="header">
      <h1>🚀 Chatlog Desktop</h1>
      <p>微信聊天记录管理工具 - 桌面版</p>
    </div>

    <div class="grid">
      <!-- 获取微信密钥 -->
      <div class="card">
        <div class="card-header">
          <div class="card-icon">🔑</div>
          <h3 class="card-title">获取微信密钥</h3>
        </div>
        <p class="card-description">
          从运行中的微信进程获取数据解密密钥
        </p>
        <button class="btn" @click="getWeChatKey">
          🔑 获取密钥
        </button>
        <div v-if="keyResult" class="result-box">
          <div class="result-title">执行结果</div>
          <div class="result-content">{{ keyResult }}</div>
        </div>
      </div>

      <!-- 解密数据库 -->
      <div class="card">
        <div class="card-header">
          <div class="card-icon">🔓</div>
          <h3 class="card-title">解密数据库</h3>
        </div>
        <div class="form-group">
          <label class="form-label">数据目录路径</label>
          <input v-model="dataDir" type="text" class="form-input" placeholder="请输入微信数据目录路径">
        </div>
        <div class="form-group">
          <label class="form-label">解密密钥</label>
          <input v-model="key" type="text" class="form-input" placeholder="请输入获取到的密钥">
        </div>
        <button class="btn" @click="decryptDatabase">
          🔓 开始解密
        </button>
        <div v-if="decryptResult" class="result-box">
          <div class="result-title">执行结果</div>
          <div class="result-content">{{ decryptResult }}</div>
        </div>
      </div>

      <!-- 启动HTTP服务 -->
      <div class="card">
        <div class="card-header">
          <div class="card-icon">🌐</div>
          <h3 class="card-title">启动HTTP服务</h3>
        </div>
        <div class="form-group">
          <label class="form-label">服务端口</label>
          <input v-model="port" type="text" class="form-input" placeholder="端口号">
        </div>
        <button class="btn" @click="startHTTPServer">
          🚀 启动服务
        </button>
        <button class="btn btn-secondary" @click="openWebInterface" style="margin-left: 10px;">
          🌐 打开网页
        </button>
        <div v-if="serverResult" class="result-box">
          <div class="result-title">执行结果</div>
          <div class="result-content">{{ serverResult }}</div>
        </div>
        
        <div class="http-info">
          <h4>📋 HTTP服务说明</h4>
          <p>启动服务后，可通过以下方式访问：</p>
          <p>• <a href="#" @click="openWebInterface">主界面</a> - 聊天记录查询</p>
          <p>• <a href="#" @click="openAPI">API接口</a> - 程序调用</p>
          <p>• <a href="#" @click="openMCP">MCP服务</a> - AI助手集成</p>
        </div>
      </div>

      <!-- 飞书同步 -->
      <div class="card">
        <div class="card-header">
          <div class="card-icon">☁️</div>
          <h3 class="card-title">飞书同步</h3>
        </div>
        <div class="form-group">
          <label class="form-label">微信账号</label>
          <input v-model="account" type="text" class="form-input" placeholder="请输入微信账号">
        </div>
        <div class="form-group">
          <label class="form-label">聊天名称</label>
          <input v-model="chatName" type="text" class="form-input" placeholder="请输入聊天名称或群名">
        </div>
        <button class="btn" @click="syncToFeishu">
          ☁️ 开始同步
        </button>
        <div v-if="feishuResult" class="result-box">
          <div class="result-title">执行结果</div>
          <div class="result-content">{{ feishuResult }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'Dashboard',
  data() {
    return {
      dataDir: '',
      key: '',
      port: '5030',
      account: '',
      chatName: '',
      keyResult: '',
      decryptResult: '',
      serverResult: '',
      feishuResult: ''
    }
  },
  methods: {
    async getWeChatKey() {
      try {
        const result = await window.go.main.App.GetWeChatKey()
        this.keyResult = result
      } catch (error) {
        console.error('Error:', error)
        this.keyResult = '执行失败: ' + error.message
      }
    },

    async decryptDatabase() {
      if (!this.dataDir || !this.key) {
        alert('请填写完整信息')
        return
      }
      
      try {
        const result = await window.go.main.App.DecryptDatabase(this.dataDir, this.key)
        this.decryptResult = result
      } catch (error) {
        console.error('Error:', error)
        this.decryptResult = '执行失败: ' + error.message
      }
    },

    async startHTTPServer() {
      try {
        const result = await window.go.main.App.StartHTTPServer(this.port)
        this.serverResult = result
      } catch (error) {
        console.error('Error:', error)
        this.serverResult = '执行失败: ' + error.message
      }
    },

    openWebInterface() {
      window.open(`http://localhost:${this.port}`, '_blank')
    },

    openAPI() {
      window.open(`http://localhost:${this.port}/api/v1/chatlog`, '_blank')
    },

    openMCP() {
      window.open(`http://localhost:${this.port}/sse`, '_blank')
    },

    async syncToFeishu() {
      if (!this.account || !this.chatName) {
        alert('请填写完整信息')
        return
      }
      
      try {
        const result = await window.go.main.App.SyncToFeishu(this.account, this.chatName)
        this.feishuResult = result
      } catch (error) {
        console.error('Error:', error)
        this.feishuResult = '执行失败: ' + error.message
      }
    }
  }
}
</script>

<style scoped>
.dashboard {
  padding: 20px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  min-height: 100vh;
}

.header {
  text-align: center;
  margin-bottom: 40px;
  color: white;
}

.header h1 {
  font-size: 32px;
  margin-bottom: 10px;
}

.header p {
  font-size: 16px;
  opacity: 0.9;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 30px;
  max-width: 1200px;
  margin: 0 auto;
}

.card {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 15px;
  padding: 30px;
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.1);
}

.card-header {
  display: flex;
  align-items: center;
  margin-bottom: 20px;
}

.card-icon {
  width: 50px;
  height: 50px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 20px;
  color: white;
  font-size: 20px;
}

.card-title {
  font-size: 20px;
  font-weight: 600;
  color: #333;
}

.card-description {
  margin-bottom: 20px;
  color: #666;
  line-height: 1.5;
}

.form-group {
  margin-bottom: 20px;
}

.form-label {
  display: block;
  margin-bottom: 8px;
  font-weight: 500;
  color: #555;
}

.form-input {
  width: 100%;
  padding: 12px 15px;
  border: 2px solid #e1e5e9;
  border-radius: 8px;
  font-size: 14px;
  transition: all 0.3s ease;
}

.form-input:focus {
  outline: none;
  border-color: #667eea;
  box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
}

.btn {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border: none;
  padding: 12px 25px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s ease;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 15px rgba(102, 126, 234, 0.3);
}

.btn-secondary {
  background: #6c757d;
}

.btn-secondary:hover {
  background: #5a6268;
}

.result-box {
  margin-top: 20px;
  padding: 15px;
  background: #f8f9fa;
  border-radius: 8px;
  border-left: 4px solid #667eea;
}

.result-title {
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
}

.result-content {
  color: #666;
  line-height: 1.5;
}

.http-info {
  background: #e3f2fd;
  border: 1px solid #2196f3;
  border-radius: 8px;
  padding: 20px;
  margin-top: 20px;
}

.http-info h4 {
  color: #1976d2;
  margin-bottom: 10px;
}

.http-info p {
  color: #424242;
  margin-bottom: 8px;
}

.http-info a {
  color: #2196f3;
  text-decoration: none;
  font-weight: 500;
  cursor: pointer;
}

.http-info a:hover {
  text-decoration: underline;
}
</style>
