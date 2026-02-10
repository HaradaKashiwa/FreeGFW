import { useState, useEffect } from "react"
import { useGetConfigs, useUpdateConfigs, useResetConfigs } from "@/apis/config"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { PiSpinner } from "react-icons/pi"
import { IoSave, IoChevronForward } from "react-icons/io5"
import { useNavigate } from "react-router-dom"
import { useLanguageStore } from "../store/useLanguageStore"

export function SettingsCard() {
    const { data: config, refresh } = useGetConfigs()
    const { trigger: updateConfigs, loading: updating } = useUpdateConfigs()
    const { trigger: resetConfigs, loading: resetting } = useResetConfigs()
    const navigate = useNavigate()
    const { t } = useLanguageStore()

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
        if (window.confirm(t('factory_reset_confirm'))) {
            await resetConfigs()
            navigate('/start')
        }
    }

    return (
        <div className='space-y-6'>
            <div className='space-y-4'>
                <h3 className="font-medium text-sm text-gray-500">{t('basic_settings')}</h3>
                <div className="grid grid-cols-1 gap-4">
                    <div>
                        <label className="block text-sm font-medium mb-1">{t('admin_username')}</label>
                        <Input
                            value={username}
                            onChange={e => setUsername(e.target.value)}
                            placeholder={t('admin_username_placeholder')}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1">{t('admin_password')}</label>
                        <Input
                            type="password"
                            value={password}
                            onChange={e => setPassword(e.target.value)}
                            placeholder={t('admin_password_placeholder')}
                        />
                    </div>
                </div>
                <div className="flex justify-end">
                    <Button disabled={updating} onClick={handleSave} size="sm" className="flex items-center gap-2">
                        {updating ? <PiSpinner className="animate-spin" /> : <IoSave />} {t('save')}
                    </Button>
                </div>
            </div>

            <div className="border-t pt-4">
                <h3 className="font-medium text-sm text-gray-500 mb-2">{t('danger_zone')}</h3>
                <div className='rounded-lg overflow-hidden border cursor-pointer bg-white hover:bg-red-50 border-red-100 transition-colors' onClick={handleReset}>
                    <div className='flex items-center justify-between p-3'>
                        <div className="flex items-center gap-2 text-red-600 font-medium">
                            {resetting ? <PiSpinner className="animate-spin" /> : null}
                            {t('factory_reset')}
                        </div>
                        <IoChevronForward className="text-red-400 rtl:rotate-180" />
                    </div>
                </div>
            </div>
        </div>
    )
}
