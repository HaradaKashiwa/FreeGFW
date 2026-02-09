import { useState, useEffect } from "react"
import { useGetConfigs, useUpdateConfigs, useResetConfigs } from "@/apis/config"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { PiSpinner } from "react-icons/pi"
import { IoSave, IoChevronForward } from "react-icons/io5"
import { useNavigate } from "react-router-dom"

export function SettingsCard() {
    const { data: config, refresh } = useGetConfigs()
    const { trigger: updateConfigs, loading: updating } = useUpdateConfigs()
    const { trigger: resetConfigs, loading: resetting } = useResetConfigs()
    const navigate = useNavigate()
    
    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')

    useEffect(() => {
        if (config) {
            setUsername(config.username || '')
            setPassword(config.password || '')
        }
    }, [config])

    const handleSave = async () => {
        await updateConfigs({ username, password })
        await refresh()
    }

    const handleReset = async () => {
        if (window.confirm('确定要恢复出厂设置吗？此操作无法撤销。')) {
            await resetConfigs()
            navigate('/start')
        }
    }

    return (
        <div className='space-y-6'>
            <div className='space-y-4'>
                 <h3 className="font-medium text-sm text-gray-500">基本设置</h3>
                 <div className="grid grid-cols-1 gap-4">
                    <div>
                        <label className="block text-sm font-medium mb-1">用户名</label>
                        <Input 
                            value={username} 
                            onChange={e => setUsername(e.target.value)} 
                            placeholder="设置访问用户名"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1">密码</label>
                        <Input 
                            type="password"
                            value={password} 
                            onChange={e => setPassword(e.target.value)} 
                            placeholder="设置访问密码"
                        />
                    </div>
                </div>
                <div className="flex justify-end">
                    <Button disabled={updating} onClick={handleSave} size="sm" className="flex items-center gap-2">
                        {updating ? <PiSpinner className="animate-spin" /> : <IoSave />} 保存
                    </Button>
                </div>
            </div>

            <div className="border-t pt-4">
                 <h3 className="font-medium text-sm text-gray-500 mb-2">危险区域</h3>
                 <div className='rounded-lg overflow-hidden border cursor-pointer bg-white hover:bg-red-50 border-red-100 transition-colors' onClick={handleReset}>
                    <div className='flex items-center justify-between p-3'>
                        <div className="flex items-center gap-2 text-red-600 font-medium">
                            {resetting ? <PiSpinner className="animate-spin" /> : null}
                            恢复出厂设置
                        </div>
                        <IoChevronForward className="text-red-400" />
                    </div>
                </div>
            </div>
        </div>
    )
}
