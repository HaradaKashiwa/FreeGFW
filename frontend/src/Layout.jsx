import { useEffect, useCallback, useState } from "react"
import { useGetConfigs, useSetTitle } from "./apis/config"
import { useNavigate } from "react-router-dom"
import { IoChevronForward, IoSettings, IoPencil, IoLanguage } from "react-icons/io5"
import { Modal } from "./components/Modal"
import { SettingsCard } from "./components/SettingsCard"
import { useLanguageStore } from "./store/useLanguageStore"
import { Button } from "@/components/ui/button"

export default function Layout({ children }) {
    const { data: configs, loaded, refresh } = useGetConfigs()
    const [editable, setEditable] = useState(false)
    const [langModalOpen, setLangModalOpen] = useState(false)
    const navigate = useNavigate()
    const { trigger: setTitle, loading: setTitleLoading, error: setTitleError } = useSetTitle()
    const { t, language, setLanguage } = useLanguageStore()

    useEffect(() => {
        if (!loaded) return
        if (!configs?.inited) navigate('/start')
        document.title = configs?.title || 'FreeGFW'
    }, [configs, loaded])

    return (
        <div className='px-2'>
            <div className='py-4 max-w-4xl mx-auto flex items-center justify-between gap-4'>
                <div className='flex items-center gap-4'>
                    <div className='text-2xl'>
                        {!editable && <div className='flex items-center gap-2 cursor-pointer' onClick={() => setEditable(true)}><span>{configs?.title || 'FreeGFW'}</span> <IoPencil /></div>}
                        {editable && <input autoFocus defaultValue={configs?.title || 'FreeGFW'} onBlur={async (e) => {
                            if (e.target.value !== configs?.title) {
                                await setTitle({
                                    title: e.target.value
                                })
                                await refresh()
                            }
                            setEditable(false)
                        }} />}
                    </div>
                </div>
                <div className='flex items-center gap-4 text-sm'>
                    <div className='cursor-pointer p-2 border rounded-full text-xl hover:bg-muted transition-colors' onClick={() => setLangModalOpen(true)}><IoLanguage /></div>
                    <Modal
                        open={langModalOpen}
                        onOpenChange={setLangModalOpen}
                        title={t('select_language')}
                        content={(
                            <div className="flex flex-col gap-2">
                                <Button variant={language === 'zh' ? 'default' : 'outline'} onClick={() => { setLanguage('zh'); setLangModalOpen(false) }}>中文</Button>
                                <Button variant={language === 'fa' ? 'default' : 'outline'} onClick={() => { setLanguage('fa'); setLangModalOpen(false) }}>فارسی</Button>
                                <Button variant={language === 'en' ? 'default' : 'outline'} onClick={() => { setLanguage('en'); setLangModalOpen(false) }}>English</Button>
                            </div>
                        )}
                    />

                    <Modal
                        content={(
                            <SettingsCard />
                        )}
                        title={t('system_settings')}
                    >
                        <div className='cursor-pointer p-2 border rounded-full text-xl hover:bg-muted transition-colors'><IoSettings /></div>
                    </Modal>
                </div>
            </div>
            <div className='max-w-4xl mx-auto'>
                {children}
            </div>
        </div>
    )
}