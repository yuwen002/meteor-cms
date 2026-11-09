<template>
  <div class="login-container">
    <el-card class="login-card">
      <h2 class="title">Meteor CMS</h2>
      <el-form :model="form" :rules="rules" ref="formRef" label-position="top">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="请输入密码" />
        </el-form-item>
        <!-- 后端可通过 LOGIN_CAPTCHA_ENABLED 关闭验证码，前端按接口返回的 enabled 自适应 -->
        <el-form-item v-if="captchaEnabled" label="验证码" prop="captcha">
          <div class="captcha-row">
            <el-input v-model="form.captcha" placeholder="请输入验证码" />
            <img :src="captchaImg" class="captcha-img" @click="refreshCaptcha" alt="验证码" />
          </div>
        </el-form-item>
        <el-button type="primary" :loading="loading" class="login-btn" @click="handleLogin">登录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from '@/utils/element'
import { login as loginApi, getCaptcha } from '@/api/auth'
import { useUserStore } from '@/stores/user'
import { setRefreshToken } from '@/utils/auth'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref<FormInstance>()
const loading = ref(false)
const captchaImg = ref('')
const captchaEnabled = ref(true)

const form = reactive({
  username: '',
  password: '',
  captcha: '',
  captcha_id: '',
})

const rules = computed<FormRules>(() => ({
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
  ...(captchaEnabled.value
    ? { captcha: [{ required: true, message: '请输入验证码', trigger: 'blur' }] }
    : {}),
}))

async function refreshCaptcha() {
  try {
    const res = await getCaptcha()
    captchaEnabled.value = res.enabled
    if (!res.enabled) {
      captchaImg.value = ''
      form.captcha = ''
      form.captcha_id = ''
      return
    }
    captchaImg.value = res.captcha_base64
    form.captcha_id = res.captcha_id
    form.captcha = ''
  } catch {
    // 拉取失败时保持验证码必填，避免用户在无验证码的情况下被后端拒绝
    captchaEnabled.value = true
  }
}

function handleLogin() {
  formRef.value?.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      const res = await loginApi(form)
      userStore.setTokenAction(res.access_token)
      setRefreshToken(res.refresh_token)
      ElMessage.success('登录成功')
      router.push('/')
    } catch {
      if (captchaEnabled.value) {
        refreshCaptcha()
      }
    } finally {
      loading.value = false
    }
  })
}

onMounted(refreshCaptcha)
</script>

<style scoped>
.login-container {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1e3c72, #2a5298);
}
.login-card {
  width: 360px;
  padding: 12px 8px;
}
.title {
  text-align: center;
  margin-bottom: 16px;
  color: #303133;
}
.captcha-row {
  display: flex;
  gap: 8px;
}
.captcha-img {
  width: 120px;
  height: 32px;
  cursor: pointer;
  border-radius: 4px;
}
.login-btn {
  width: 100%;
}
</style>
